import { Injectable } from '@nestjs/common';
import { Grade } from '@prisma/client';
import { GRADE_ORDER, MatchScore, Need, ScoreableCandidate, WEIGHTS, normalizeTech } from './scoring.service';

export interface ExplanationDetail {
  factor: string;
  match: boolean | string;
  weight: number;
  explanation: string;
  overlap?: string[];
  detail?: string;
}

export interface MatchExplanation {
  overall: string;
  details: ExplanationDetail[];
}

@Injectable()
export class ExplanationService {
  generate(
    candidate: ScoreableCandidate & { firstName?: string | null; lastName?: string | null; lastTestDate?: Date | null; achievementNames?: string[] },
    need: Need,
    score: MatchScore,
  ): MatchExplanation {
    const details: ExplanationDetail[] = [];

    // Специализация
    if (!need.specialization) {
      details.push({
        factor: 'Специализация',
        match: 'не указана',
        weight: WEIGHTS.category,
        explanation: 'В потребности специализация не задана — нейтральный балл',
      });
    } else if (candidate.specialization === need.specialization) {
      details.push({
        factor: 'Специализация',
        match: true,
        weight: WEIGHTS.category,
        explanation: `Совпадает: ${candidate.specialization}`,
      });
    } else {
      details.push({
        factor: 'Специализация',
        match: false,
        weight: WEIGHTS.category,
        explanation: `Кандидат — ${candidate.specialization ?? 'не указана'}, требуется ${need.specialization}`,
      });
    }

    // Стек
    const overlap = (candidate.stack || []).filter((s) =>
      (need.stack || []).some((n) => normalizeTech(n) === normalizeTech(s)),
    );
    if (need.stack?.length) {
      details.push({
        factor: 'Стек',
        match: overlap.length > 0,
        weight: WEIGHTS.stack,
        overlap,
        explanation: `Совпадение ${overlap.length}/${need.stack.length}: ${overlap.join(', ') || 'нет пересечений'}`,
      });
    }

    // Грейд
    const candidateGrade: Grade | null = candidate.verifiedGrade || candidate.declaredGrade;
    if (need.grade) {
      const idxDiff = gradeIndex(candidateGrade) - gradeIndex(need.grade);
      let explanation: string;
      if (candidateGrade === need.grade) explanation = `Грейд ${need.grade} ${candidate.verifiedGrade ? 'подтверждён тестом' : 'заявлен кандидатом'}`;
      else if (idxDiff === 1) explanation = `Грейд выше запросленного (${candidateGrade} vs ${need.grade}) — допустимо`;
      else if (idxDiff === -1) explanation = `Грейд ниже запросленного (${candidateGrade ?? 'не указан'} vs ${need.grade}) — допустимо`;
      else if (idxDiff > 1) explanation = `Избыточно высокий грейд: ${candidateGrade} при запросе ${need.grade}`;
      else explanation = `Грейд ниже запросленного: ${candidateGrade ?? 'не указан'} при запросе ${need.grade}`;

      details.push({
        factor: 'Грейд',
        match: candidateGrade === need.grade,
        weight: WEIGHTS.grade,
        explanation,
      });
    }

    // Семантика
    details.push({
      factor: 'Семантическое совпадение',
      match: score.components.semanticSimilarity >= 50,
      weight: WEIGHTS.semantic,
      detail: `${score.components.semanticSimilarity}%`,
      explanation:
        score.components.semanticSimilarity >= 50
          ? 'Описание потребности близко к описанию профиля'
          : 'Описание потребности слабо коррелирует с профилем',
    });

    // Достижения ФСП
    if (candidate.achievementNames?.length) {
      details.push({
        factor: 'Достижения ФСП',
        match: true,
        weight: WEIGHTS.achievement,
        detail: candidate.achievementNames.slice(0, 2).join(', '),
        explanation: `Подтверждённые достижения: ${candidate.achievementNames.slice(0, 2).join(', ')}`,
      });
    }

    // Активность
    if (candidate.lastTestDate) {
      const days = Math.max(0, Math.round((Date.now() - candidate.lastTestDate.getTime()) / 86_400_000));
      details.push({
        factor: 'Активность',
        match: days <= 90,
        weight: 0,
        detail: `тест пройден ${days} дн. назад`,
        explanation: days <= 90 ? 'Кандидат недавно проходил тестирование' : 'Тестирование было давно',
      });
    }

    return { overall: this.summary(score.total), details };
  }

  summary(total: number): string {
    if (total >= 80) return 'Сильное совпадение по ключевым факторам';
    if (total >= 60) return 'Хорошее совпадение с некоторыми расхождениями';
    if (total >= 40) return 'Частичное совпадение';
    return 'Слабое совпадение';
  }
}

function gradeIndex(grade: Grade | null): number {
  return grade ? GRADE_ORDER.indexOf(grade) : -1;
}
