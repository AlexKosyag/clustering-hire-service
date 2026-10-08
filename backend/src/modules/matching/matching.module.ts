import { Module } from '@nestjs/common';
import { EmbeddingService } from './embedding.service';
import { ExplanationService } from './explanation.service';
import { IndexingService } from './indexing.service';
import { MatchingController } from './matching.controller';
import { MatchingScoringService } from './scoring.service';
import { MatchingService } from './matching.service';
import { VectorRepository } from './vector.repository';

@Module({
  controllers: [MatchingController],
  providers: [
    MatchingService,
    MatchingScoringService,
    ExplanationService,
    IndexingService,
    EmbeddingService,
    VectorRepository,
  ],
  exports: [MatchingService, IndexingService, MatchingScoringService, ExplanationService, EmbeddingService],
})
export class MatchingModule {}
