import { Body, Controller, Get, Post } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { EmployerNeedDto, SearchCandidatesDto, SemanticQueryDto } from './dto/matching.dto';
import { MatchingService } from './matching.service';
import { MatchingServiceV2 } from './matching-v2.service';

@ApiTags('Matching')
@ApiBearerAuth('bearer')
@Controller('matching')
export class MatchingController {
  constructor(
    private readonly matching: MatchingService,
    private readonly matchingV2: MatchingServiceV2,
  ) {}

  // ─────────────────────────── V1 Endpoints (legacy) ───────────────────────────

  @Post('search')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Подбор кандидатов (V1 — legacy)' })
  search(@Body() dto: SearchCandidatesDto) {
    return this.matching.search(dto);
  }

  @Post('semantic')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Семантический поиск (V1 — legacy)' })
  semantic(@Body() dto: SemanticQueryDto) {
    return this.matching.semanticSearch(dto);
  }

  @Get('recommendations')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Рекомендации кандидатов (V1 — legacy)' })
  recommendations(@CurrentUser() user: AuthUser) {
    return this.matching.recommendations(user.id);
  }

  // ─────────────────────────── V2 Endpoints ───────────────────────────

  @Post('v2/search')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({
    summary: 'Подбор кандидатов V2 (ML query understanding + expanded scoring + re-ranking)',
    description:
      'Использует ML для понимания запроса, расширенный scoring (7 features), ' +
      'опциональный cross-encoder re-ranking, детальную объяснимость.',
  })
  searchV2(@Body() dto: SearchCandidatesDto) {
    return this.matchingV2.search(dto);
  }

  @Post('v2/semantic')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({
    summary: 'Семантический поиск V2 (ML embeddings + query understanding)',
  })
  semanticV2(@Body() dto: SemanticQueryDto) {
    return this.matchingV2.semanticSearch(dto);
  }

  @Post('v2/need')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({
    summary: 'Создать потребность V2 (query understanding + подборка)',
    description: 'Анализирует описание потребности через ML и формирует подборку.',
  })
  createNeedV2(
    @CurrentUser() user: AuthUser,
    @Body() dto: EmployerNeedDto,
  ) {
    return this.matchingV2.createNeed(user.id, dto);
  }
}