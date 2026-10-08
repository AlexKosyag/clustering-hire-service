import { Body, Controller, Get, Post } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { EmployerNeedDto, SearchCandidatesDto, SemanticQueryDto } from './dto/matching.dto';
import { MatchingService } from './matching.service';

@ApiTags('Matching')
@ApiBearerAuth('bearer')
@Controller('matching')
export class MatchingController {
  constructor(private readonly matching: MatchingService) {}

  @Post('search')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Подбор кандидатов по потребности (semantic + category ranking)' })
  search(@Body() dto: SearchCandidatesDto) {
    return this.matching.search(dto);
  }

  @Post('semantic')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Семантический поиск по описанию' })
  semantic(@Body() dto: SemanticQueryDto) {
    return this.matching.semanticSearch(dto);
  }

  @Get('recommendations')
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Рекомендации кандидатов (collaborative filtering)' })
  recommendations(@CurrentUser() user: AuthUser) {
    return this.matching.recommendations(user.id);
  }
}
