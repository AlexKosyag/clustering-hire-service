import { Body, Controller, Get, Param, Post } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { RetakeDto, StartTestDto, SubmitAnswerDto } from './dto/testing.dto';
import { TestingService } from './testing.service';

@ApiTags('Testing')
@ApiBearerAuth('bearer')
@Controller('testing')
@Roles(UserRole.CANDIDATE)
export class TestingController {
  constructor(private readonly testing: TestingService) {}

  @Post('start')
  @ApiOperation({ summary: 'Начать адаптивное тестирование (CAT + IRT)' })
  start(@CurrentUser() user: AuthUser, @Body() dto: StartTestDto) {
    return this.testing.start(user.id, dto);
  }

  @Post('retake')
  @ApiOperation({ summary: 'Пересдача (с учётом cooldown 3 месяца)' })
  retake(@CurrentUser() user: AuthUser, @Body() dto: RetakeDto) {
    return this.testing.retake(user.id, dto.specialization);
  }

  @Get('active')
  @ApiOperation({ summary: 'Активная сессия тестирования (для восстановления)' })
  active(@CurrentUser() user: AuthUser) {
    return this.testing.active(user.id);
  }

  @Get('session/:id/current')
  @ApiOperation({ summary: 'Текущий вопрос сессии' })
  current(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testing.current(user.id, id);
  }

  @Post('session/:id/answer')
  @ApiOperation({ summary: 'Отправить ответ (возвращает следующий вопрос — CAT)' })
  answer(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: SubmitAnswerDto) {
    return this.testing.answer(user.id, id, dto);
  }

  @Post('session/:id/complete')
  @ApiOperation({ summary: 'Завершить тест: score, грейд, рекомендация, cooldown' })
  complete(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testing.complete(user.id, id);
  }
}
