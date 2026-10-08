import { Module } from '@nestjs/common';
import { MatchingModule } from '../matching/matching.module';
import { AntiCheatService } from './services/anti-cheat.service';
import { CATService } from './services/cat.service';
import { QuestionPoolService } from './services/question-pool.service';
import { ScoringService } from './services/scoring.service';
import { TestingController } from './testing.controller';
import { TestingService } from './testing.service';

@Module({
  imports: [MatchingModule],
  controllers: [TestingController],
  providers: [TestingService, QuestionPoolService, CATService, ScoringService, AntiCheatService],
  exports: [TestingService, ScoringService],
})
export class TestingModule {}
