import { Module } from '@nestjs/common';
import { HttpModule } from '@nestjs/axios';
import { EmbeddingService } from './embedding.service';
import { ExplanationService } from './explanation.service';
import { IndexingService } from './indexing.service';
import { MatchingController } from './matching.controller';
import { MatchingScoringService } from './scoring.service';
import { MatchingScoringServiceV2 } from './scoring-v2.service';
import { MatchingService } from './matching.service';
import { MatchingServiceV2 } from './matching-v2.service';
import { VectorRepository } from './vector.repository';
import { MlServiceClient } from './ml-client.service';
import { QueryUnderstandingService } from './query-understanding.service';

@Module({
  imports: [HttpModule],
  controllers: [MatchingController],
  providers: [
    // V1
    MatchingService,
    MatchingScoringService,
    // V2
    MatchingServiceV2,
    MatchingScoringServiceV2,
    MlServiceClient,
    QueryUnderstandingService,
    // Shared
    ExplanationService,
    IndexingService,
    EmbeddingService,
    VectorRepository,
  ],
  exports: [
    MatchingService,
    MatchingServiceV2,
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