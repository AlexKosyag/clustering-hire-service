import { Controller, Get, Param, Query } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { GamificationService } from './gamification.service';

@ApiTags('Gamification')
@ApiBearerAuth('bearer')
@Controller('gamification')
export class GamificationController {
  constructor(private readonly gamification: GamificationService) {}

  @Get('profile')
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Профиль геймификации (XP, уровень, стрик)' })
  getProfile(@CurrentUser() user: AuthUser) {
    return this.gamification.getProfile(user.id);
  }

  @Get('badges')
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Все бейджи с earned статусом' })
  getBadges(@CurrentUser() user: AuthUser) {
    return this.gamification.getBadges(user.id);
  }

  @Get('leaderboard')
  @ApiOperation({ summary: 'Рейтинг кандидатов' })
  getLeaderboard(
    @Query('type') type?: 'global' | 'specialization',
    @Query('specialization') specialization?: string,
    @Query('limit') limit?: string,
  ) {
    return this.gamification.getLeaderboard(
      type || 'global',
      specialization,
      parseInt(limit || '20', 10),
    );
  }

  @Get('xp-history')
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'История начисления XP' })
  getXpHistory(@CurrentUser() user: AuthUser, @Query('limit') limit?: string) {
    return this.gamification.getXpHistory(user.id, parseInt(limit || '50', 10));
  }
}