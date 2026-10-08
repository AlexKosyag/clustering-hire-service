import { Module } from '@nestjs/common';
import { MatchingModule } from '../matching/matching.module';
import { StorageModule } from '../storage/storage.module';
import { EmployersController } from './employers.controller';
import { EmployersService } from './employers.service';

@Module({
  imports: [MatchingModule, StorageModule],
  controllers: [EmployersController],
  providers: [EmployersService],
  exports: [EmployersService],
})
export class EmployersModule {}
