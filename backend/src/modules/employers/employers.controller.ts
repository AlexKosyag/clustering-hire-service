import { Body, Controller, Get, Param, Post, Put, Query, UploadedFile, UseInterceptors } from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { ApiBearerAuth, ApiConsumes, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { EmployerNeedDto } from '../matching/dto/matching.dto';
import { CreateNeedDto, EmployerSearchQueryDto, UpdateEmployerProfileDto } from './dto/employer.dto';
import { EmployersService } from './employers.service';

@ApiTags('Employers')
@ApiBearerAuth('bearer')
@Controller('employers')
@Roles(UserRole.EMPLOYER)
export class EmployersController {
  constructor(private readonly employers: EmployersService) {}

  @Get('profile')
  @ApiOperation({ summary: 'Профиль компании' })
  profile(@CurrentUser() user: AuthUser) {
    return this.employers.profile(user.id);
  }

  @Put('profile')
  @ApiOperation({ summary: 'Обновить профиль компании' })
  update(@CurrentUser() user: AuthUser, @Body() dto: UpdateEmployerProfileDto) {
    return this.employers.updateProfile(user.id, dto);
  }

  @Post('profile/logo')
  @UseInterceptors(FileInterceptor('file'))
  @ApiConsumes('multipart/form-data')
  @ApiOperation({ summary: 'Загрузить логотип компании' })
  uploadLogo(@CurrentUser() user: AuthUser, @UploadedFile() file: any) {
    return this.employers.uploadLogo(user.id, file);
  }

  @Post('need')
  @ApiOperation({ summary: 'Описать потребность и получить подборку кандидатов' })
  createNeed(@CurrentUser() user: AuthUser, @Body() dto: EmployerNeedDto) {
    return this.employers.createNeed(user.id, dto);
  }

  @Get('candidates/search')
  @ApiOperation({ summary: 'Поиск по базе кандидатов (фильтры + семантика)' })
  search(@CurrentUser() user: AuthUser, @Query() query: EmployerSearchQueryDto) {
    return this.employers.searchCandidates(user.id, query);
  }

  @Get('candidates/:id')
  @ApiOperation({ summary: 'Профиль кандидата (контакты скрыты до ACCEPTED)' })
  candidate(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.employers.candidateDetail(user.id, id);
  }

  @Get('dashboard')
  @ApiOperation({ summary: 'Дашборд работодателя' })
  dashboard(@CurrentUser() user: AuthUser) {
    return this.employers.dashboard(user.id);
  }
}
