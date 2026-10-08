import { Module } from '@nestjs/common';
import { HttpModule } from '@nestjs/axios';
import { MatchingModule } from '../matching/matching.module';
import { GamificationModule } from '../gamification/gamification.module';
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
import { TestingServiceV2 } from './testing-v2.service';

@Module({
  imports: [MatchingModule, GamificationModule, HttpModule],
  controllers: [TestingController],
  providers: [
    // V1
    TestingService,
    QuestionPoolService,
    CATService,
    ScoringService,
    AntiCheatService,
    // V2
    TestingServiceV2,
    BehavioralAntiCheatService,
    CodeSandboxService,
    LLMEvaluationService,
    TaskGeneratorService,
  ],
  exports: [
    TestingService,
    TestingServiceV2,
    ScoringService,
    BehavioralAntiCheatService,
    CodeSandboxService,
    LLMEvaluationService,
    TaskGeneratorService,
  ],
})
export class TestingModule {}