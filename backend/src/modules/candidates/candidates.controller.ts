import { Body, Controller, Get, Post, Put, UploadedFile, UseInterceptors } from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { ApiBearerAuth, ApiConsumes, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { CandidatesService } from './candidates.service';
import { UpdateCandidateProfileDto } from './dto/candidate.dto';

@ApiTags('Candidates')
@ApiBearerAuth('bearer')
@Controller('candidates')
@Roles(UserRole.CANDIDATE)
export class CandidatesController {
  constructor(private readonly candidates: CandidatesService) {}

  @Get('profile')
  @ApiOperation({ summary: 'Профиль кандидата' })
  profile(@CurrentUser() user: AuthUser) {
    return this.candidates.profile(user.id);
  }

  @Put('profile')
  @ApiOperation({ summary: 'Обновить профиль (переиндексация для matching)' })
  update(@CurrentUser() user: AuthUser, @Body() dto: UpdateCandidateProfileDto) {
    return this.candidates.updateProfile(user.id, dto);
  }

  @Get('category')
  @ApiOperation({ summary: 'Текущая категория/грейд и статус тестирования' })
  category(@CurrentUser() user: AuthUser) {
    return this.candidates.category(user.id);
  }

  @Post('profile/resume')
  @UseInterceptors(FileInterceptor('file'))
  @ApiConsumes('multipart/form-data')
  @ApiOperation({ summary: 'Загрузить резюме (PDF)' })
  uploadResume(@CurrentUser() user: AuthUser, @UploadedFile() file: any) {
    return this.candidates.uploadResume(user.id, file);
  }

  @Post('profile/avatar')
  @UseInterceptors(FileInterceptor('file'))
  @ApiConsumes('multipart/form-data')
  @ApiOperation({ summary: 'Загрузить аватар' })
  uploadAvatar(@CurrentUser() user: AuthUser, @UploadedFile() file: any) {
    return this.candidates.uploadAvatar(user.id, file);
  }
}
