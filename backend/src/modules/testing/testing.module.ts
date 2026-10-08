import { Module } from '@nestjs/common';
import { HttpModule } from '@nestjs/axios';
import { MatchingModule } from '../matching/matching.module';
import { AntiCheatService } from './services/anti-cheat.service';
import { BehavioralAntiCheatService } from './services/behavioral-anti-cheat.service';
import { CATService } from './services/cat.service';
import { CodeSandboxService } from './services/code-sandbox.service';
import { LLMEvaluationService } from './services/llm-evaluation.service';
import { QuestionPoolService } from './services/question-pool.service';
import { ScoringService } from './services/scoring.service';
import { TaskGeneratorService } from './services/task-generator.service';
import { TestingController } from './testing.controller';
import { TestingService } from './testing.service';

@Module({
  imports: [MatchingModule, HttpModule],
  controllers: [TestingController],
  providers: [
    TestingService,
    QuestionPoolService,
    CATService,
    ScoringService,
    AntiCheatService,
    BehavioralAntiCheatService,
    CodeSandboxService,
    LLMEvaluationService,
    TaskGeneratorService,
  ],
  exports: [
    TestingService,
    ScoringService,
    BehavioralAntiCheatService,
    CodeSandboxService,
    LLMEvaluationService,
    TaskGeneratorService,
  ],
})
export class TestingModule {}