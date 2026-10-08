import { Body, Controller, Get, Param, Post } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { RetakeDto, StartTestDto, SubmitAnswerDto } from './dto/testing.dto';
import { TestingService } from './testing.service';
import { TestingServiceV2 } from './testing-v2.service';

@ApiTags('Testing')
@ApiBearerAuth('bearer')
@Controller('testing')
@Roles(UserRole.CANDIDATE)
export class TestingController {
  constructor(
    private readonly testing: TestingService,
    private readonly testingV2: TestingServiceV2,
  ) {}

  // ─────────────────────────── V1 Endpoints (legacy) ───────────────────────────

  @Post('start')
  @ApiOperation({ summary: 'Начать тестирование (V1 — legacy)' })
  start(@CurrentUser() user: AuthUser, @Body() dto: StartTestDto) {
    return this.testing.start(user.id, dto);
  }

  @Post('retake')
  @ApiOperation({ summary: 'Пересдача (V1 — legacy)' })
  retake(@CurrentUser() user: AuthUser, @Body() dto: RetakeDto) {
    return this.testing.retake(user.id, dto.specialization);
  }

  @Get('active')
  @ApiOperation({ summary: 'Активная сессия (V1 — legacy)' })
  active(@CurrentUser() user: AuthUser) {
    return this.testing.active(user.id);
  }

  @Get('session/:id/current')
  @ApiOperation({ summary: 'Текущий вопрос (V1 — legacy)' })
  current(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testing.current(user.id, id);
  }

  @Post('session/:id/answer')
  @ApiOperation({ summary: 'Отправить ответ (V1 — legacy)' })
  answer(@CurrentUser() user: AuthUser, @Param('id') id: string, @Body() dto: SubmitAnswerDto) {
    return this.testing.answer(user.id, id, dto);
  }

  @Post('session/:id/complete')
  @ApiOperation({ summary: 'Завершить тест (V1 — legacy)' })
  complete(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testing.complete(user.id, id);
  }

  // ─────────────────────────── V2 Endpoints ───────────────────────────

  @Post('v2/start')
  @ApiOperation({
    summary: 'Начать тестирование V2 (мультиформатные задания)',
    description:
      'Поддерживает: multiple_choice, code_execution, llm_open, code_review, system_design. ' +
      'Распределение заданий зависит от грейда.',
  })
  startV2(@CurrentUser() user: AuthUser, @Body() dto: StartTestDto) {
    return this.testingV2.start(user.id, dto);
  }

  @Get('v2/session/:id/current')
  @ApiOperation({ summary: 'Текущая задача V2' })
  currentV2(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testingV2.currentTask(user.id, id);
  }

  @Post('v2/session/:id/answer')
  @ApiOperation({
    summary: 'Отправить ответ V2 (с behavioral данными для anti-cheat)',
    description: 'Поддерживает behavioral data: copy-paste, tab-switch, keystroke count.',
  })
  answerV2(
    @CurrentUser() user: AuthUser,
    @Param('id') id: string,
    @Body() dto: SubmitAnswerDto & { behavioral?: any },
  ) {
    return this.testingV2.submitAnswer(user.id, id, dto);
  }

  @Post('v2/session/:id/complete')
  @ApiOperation({
    summary: 'Завершить тест V2 (task breakdown + behavioral anti-cheat)',
  })
  completeV2(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.testingV2.complete(user.id, id);
  }
}