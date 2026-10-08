import { Module } from '@nestjs/common';
import { MatchingModule } from '../matching/matching.module';
import { StorageModule } from '../storage/storage.module';
import { CandidatesController } from './candidates.controller';
import { CandidatesService } from './candidates.service';
import { PdfParsingService } from './pdf-parsing.service';

@Module({
  imports: [MatchingModule, StorageModule],
  controllers: [CandidatesController],
  providers: [CandidatesService, PdfParsingService],
  exports: [CandidatesService, PdfParsingService],
})
export class CandidatesModule {}