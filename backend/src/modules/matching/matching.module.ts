import { Module } from '@nestjs/common';
import { HttpModule } from '@nestjs/axios';
import { EmbeddingService } from './embedding.service';
import { ExplanationService } from './explanation.service';
import { IndexingService } from './indexing.service';
import { MatchingController } from './matching.controller';
import { MatchingScoringService } from './scoring.service';
import { MatchingScoringServiceV2 } from './scoring-v2.service';
import { MatchingService } from './matching.service';
import { VectorRepository } from './vector.repository';
import { MlServiceClient } from './ml-client.service';
import { QueryUnderstandingService } from './query-understanding.service';

@Module({
  imports: [HttpModule],
  controllers: [MatchingController],
  providers: [
    MatchingService,
    MatchingScoringService,
    MatchingScoringServiceV2,
    ExplanationService,
    IndexingService,
    EmbeddingService,
    VectorRepository,
    MlServiceClient,
    QueryUnderstandingService,
  ],
  exports: [
    MatchingService,
    IndexingService,
    MatchingScoringService,
    MatchingScoringServiceV2,
    ExplanationService,
    EmbeddingService,
    MlServiceClient,
    QueryUnderstandingService,
  ],
})
export class MatchingModule {}