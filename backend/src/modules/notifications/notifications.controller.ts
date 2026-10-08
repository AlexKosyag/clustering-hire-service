import { Controller, Get, Query } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiQuery, ApiTags } from '@nestjs/swagger';
import { Type } from 'class-transformer';
import { IsInt, IsOptional, Max, Min } from 'class-validator';
import { AuthUser, CurrentUser } from '../../common/decorators/auth.decorator';
import { NotificationsService } from './notifications.service';

@ApiTags('Notifications')
@ApiBearerAuth('bearer')
@Controller('notifications')
export class NotificationsController {
  constructor(private readonly notifications: NotificationsService) {}

  @Get()
  @ApiOperation({ summary: 'Уведомления пользователя (приглашения, отклики, тесты)' })
  @ApiQuery({ name: 'limit', required: false })
  list(
    @CurrentUser() user: AuthUser,
    @Query('limit') limit?: number,
  ) {
    const value = Math.min(Math.max(Number(limit) || 20, 1), 100);
    return this.notifications.list(user.id, user.role, value);
  }
}
