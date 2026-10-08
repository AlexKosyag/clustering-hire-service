import { Module } from '@nestjs/common';
import { MatchingModule } from '../matching/matching.module';
import { FspController } from './fsp.controller';
import { FspService } from './fsp.service';

@Module({
  imports: [MatchingModule],
  controllers: [FspController],
  providers: [FspService],
  exports: [FspService],
})
export class FspModule {}
