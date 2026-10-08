import { Body, Controller, Delete, Get, HttpCode, Param, Post, Put, Query } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiQuery, ApiTags } from '@nestjs/swagger';
import { Grade, Specialization, UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Public, Roles } from '../../common/decorators/auth.decorator';
import { CreateVacancyDto, UpdateVacancyDto } from './dto/vacancy.dto';
import { VacanciesService } from './vacancies.service';

@ApiTags('Vacancies')
@Controller('vacancies')
export class VacanciesController {
  constructor(private readonly vacancies: VacanciesService) {}

  @Public()
  @Get()
  @ApiOperation({ summary: 'Список вакансий (фильтры: специализация, грейд, поиск)' })
  @ApiQuery({ name: 'specialization', required: false, enum: Specialization })
  @ApiQuery({ name: 'grade', required: false, enum: Grade })
  @ApiQuery({ name: 'search', required: false })
  @ApiQuery({ name: 'limit', required: false })
  @ApiQuery({ name: 'offset', required: false })
  list(@Query() query: Record<string, string>) {
    return this.vacancies.list(query as any);
  }

  @Public()
  @Get(':id')
  @ApiOperation({ summary: 'Вакансия по ID' })
  get(@Param('id') id: string) {
    return this.vacancies.get(id);
  }

  @Post()
  @ApiBearerAuth('bearer')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Создать вакансию' })
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateVacancyDto) {
    return this.vacancies.create(user.id, dto);
  }

  @Put(':id')
  @ApiBearerAuth('bearer')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Обновить вакансию' })
  update(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: UpdateVacancyDto) {
    return this.vacancies.update(user.id, id, dto);
  }

  @Delete(':id')
  @ApiBearerAuth('bearer')
  @Roles(UserRole.EMPLOYER)
  @HttpCode(200)
  @ApiOperation({ summary: 'Удалить вакансию' })
  remove(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.vacancies.remove(user.id, id);
  }
}
