import { Injectable, Logger } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';
import { firstValueFrom } from 'rxjs';
import { AxiosResponse } from 'axios';

interface OpenAIResponse {
  choices: Array<{
    message: {
      content: string;
    };
  }>;
}

export interface LLMEvaluationCriteria {
  name: string;
  weight: number;
  description?: string;
}

export interface LLMEvaluationResult {
  score: number;  // 0-100
  breakdown: Record<string, number>;  // criteria name → score
  strengths: string[];
  weaknesses: string[];
  feedback: string;
  provider: string;
  latencyMs: number;
}

export interface LLMEvaluationRequest {
  question: string;
  candidateAnswer: string;
  criteria: LLMEvaluationCriteria[];
  referenceAnswer?: string;
  keyConcepts?: string[];
  maxTokens?: number;
}

/**
 * V2: LLM Evaluation Service.
 *
 * Evaluates open-text answers using LLM with structured criteria.
 * Falls back to keyword matching when LLM unavailable.
 */
@Injectable()
export class LLMEvaluationService {
  private readonly logger = new Logger('LLMEvaluation');
  private readonly apiKey: string | null;
  private readonly apiUrl: string;
  private readonly model: string;

  constructor(private readonly http: HttpService) {
    this.apiKey = process.env.OPENAI_API_KEY || null;
    this.apiUrl = process.env.OPENAI_API_URL || 'https://api.openai.com/v1/chat/completions';
    this.model = process.env.OPENAI_MODEL || 'gpt-4o-mini';

    if (this.apiKey) {
      this.logger.log(`LLM Evaluation enabled — model=${this.model}`);
    } else {
      this.logger.warn('LLM Evaluation: OPENAI_API_KEY not set, using keyword fallback');
    }
  }

  get available(): boolean {
    return !!this.apiKey;
  }

  /**
   * Evaluate candidate answer using LLM.
   */
  async evaluate(req: LLMEvaluationRequest): Promise<LLMEvaluationResult> {
    const t0 = Date.now();

    if (this.available) {
      try {
        return await this.evaluateWithLLM(req, t0);
      } catch (e) {
        this.logger.warn(`LLM evaluation failed: ${(e as Error).message}, using fallback`);
      }
    }

    // Fallback: keyword matching
    return this.evaluateWithKeywords(req, t0);
  }

  /**
   * Evaluate using LLM (OpenAI API).
   */
  private async evaluateWithLLM(
    req: LLMEvaluationRequest,
    t0: number,
  ): Promise<LLMEvaluationResult> {
    const criteriaText = req.criteria
      .map((c) => `- ${c.name} (weight: ${c.weight}): ${c.description || c.name}`)
      .join('\n');

    const keyConceptsText = req.keyConcepts?.length
      ? `\nKey concepts to look for: ${req.keyConcepts.join(', ')}`
      : '';

    const referenceText = req.referenceAnswer
      ? `\nReference answer (for context, not exact match required):\n${req.referenceAnswer}`
      : '';

    const prompt = `You are an expert technical interviewer evaluating a candidate's answer.

QUESTION:
${req.question}

CANDIDATE'S ANSWER:
${req.candidateAnswer}

EVALUATION CRITERIA:
${criteriaText}${keyConceptsText}${referenceText}

Evaluate the answer based on the criteria. Consider:
1. Technical accuracy
2. Completeness
3. Clarity and structure
4. Practical examples (if applicable)

Return ONLY a JSON object with this exact structure:
{
  "score": <0-100 overall score>,
  "breakdown": { "<criterion_name>": <0-100 score>, ... },
  "strengths": ["<strength 1>", "<strength 2>"],
  "weaknesses": ["<weakness 1>", "<weakness 2>"],
  "feedback": "<2-3 sentence constructive feedback>"
}`;

    const response = await firstValueFrom<AxiosResponse<OpenAIResponse>>(
      this.http.post<OpenAIResponse>(
        this.apiUrl,
        {
          model: this.model,
          messages: [
            {
              role: 'system',
              content: 'You are a technical assessment expert. Always respond with valid JSON.',
            },
            { role: 'user', content: prompt },
          ],
          temperature: 0.3,
          max_tokens: req.maxTokens || 1000,
        },
        {
          headers: {
            Authorization: `Bearer ${this.apiKey}`,
            'Content-Type': 'application/json',
          },
          timeout: 30000,
        },
      ),
    );

    const content = response.data.choices[0].message.content;
    const parsed = JSON.parse(content);

    return {
      score: Math.max(0, Math.min(100, parsed.score)),
      breakdown: parsed.breakdown || {},
      strengths: parsed.strengths || [],
      weaknesses: parsed.weaknesses || [],
      feedback: parsed.feedback || '',
      provider: `openai:${this.model}`,
      latencyMs: Date.now() - t0,
    };
  }

  /**
   * Fallback: keyword-based evaluation.
   */
  private evaluateWithKeywords(
    req: LLMEvaluationRequest,
    t0: number,
  ): Promise<LLMEvaluationResult> {
    const answerLower = req.candidateAnswer.toLowerCase();
    const breakdown: Record<string, number> = {};

    // Check for key concepts
    const foundConcepts: string[] = [];
    const missingConcepts: string[] = [];

    for (const concept of req.keyConcepts || []) {
      if (answerLower.includes(concept.toLowerCase())) {
        foundConcepts.push(concept);
      } else {
        missingConcepts.push(concept);
      }
    }

    // Score based on concept coverage
    const totalConcepts = (req.keyConcepts || []).length;
    const conceptScore =
      totalConcepts > 0 ? (foundConcepts.length / totalConcepts) * 100 : 50;

    // Score based on answer length (very rough)
    const wordCount = req.candidateAnswer.split(/\s+/).length;
    const lengthScore = Math.min(100, wordCount * 2);

    // Score based on structure (has bullet points, numbers, etc.)
    const hasStructure = /[-•*\d]/.test(req.candidateAnswer);
    const structureScore = hasStructure ? 70 : 40;

    // Calculate per-criterion scores
    for (const criterion of req.criteria) {
      switch (criterion.name.toLowerCase()) {
        case 'accuracy':
        case 'точность':
          breakdown[criterion.name] = conceptScore;
          break;
        case 'completeness':
        case 'полнота':
          breakdown[criterion.name] = conceptScore * 0.8 + lengthScore * 0.2;
          break;
        case 'structure':
        case 'структура':
          breakdown[criterion.name] = structureScore;
          break;
        default:
          breakdown[criterion.name] = (conceptScore + lengthScore) / 2;
      }
    }

    // Weighted overall score
    const totalWeight = req.criteria.reduce((sum, c) => sum + c.weight, 0);
    let overallScore = 0;
    if (totalWeight > 0) {
      for (const criterion of req.criteria) {
        overallScore += (breakdown[criterion.name] || 50) * (criterion.weight / totalWeight);
      }
    } else {
      overallScore = conceptScore;
    }

    return Promise.resolve({
      score: Math.round(overallScore),
      breakdown,
      strengths: foundConcepts.slice(0, 3),
      weaknesses: missingConcepts.slice(0, 3),
      feedback: `Found ${foundConcepts.length}/${totalConcepts} key concepts. ${
        missingConcepts.length > 0
          ? `Missing: ${missingConcepts.slice(0, 3).join(', ')}`
          : 'Good coverage!'
      }`,
      provider: 'keyword-fallback',
      latencyMs: Date.now() - t0,
    });
  }
}