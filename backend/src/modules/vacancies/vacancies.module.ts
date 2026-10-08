import { Module } from '@nestjs/common';
import { ApplicationsController, VacanciesController } from './vacancies.controller';
import { VacanciesService } from './vacancies.service';

@Module({
  controllers: [VacanciesController, ApplicationsController],
  providers: [VacanciesService],
  exports: [VacanciesService],
})
export class VacanciesModule {}
