import { Body, Controller, Get, HttpCode, Param, Post, Query } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiQuery, ApiTags } from '@nestjs/swagger';
import { InvitationStatus, UserRole } from '@prisma/client';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { CreateInvitationDto, InvitationStatusDto } from './dto/invitation.dto';
import { InvitationsService } from './invitations.service';

@ApiTags('Invitations')
@ApiBearerAuth('bearer')
@Controller('invitations')
export class InvitationsController {
  constructor(private readonly invitations: InvitationsService) {}

  @Post()
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Пригласить кандидата (EMPLOYER-first)' })
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateInvitationDto) {
    return this.invitations.create(user.id, dto);
  }

  @Get()
  @ApiOperation({ summary: 'Мои приглашения (входящие/исходящие по роли)' })
  @ApiQuery({ name: 'status', required: false, enum: InvitationStatus })
  list(@CurrentUser() user: AuthUser, @Query('status') status?: InvitationStatus) {
    return this.invitations.list(user.id, user.role, status);
  }

  @Get(':id')
  @ApiOperation({ summary: 'Детали приглашения (контакты — после ACCEPTED)' })
  get(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.invitations.get(user.id, user.role, id);
  }

  @Post(':id/accept')
  @HttpCode(200)
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Принять приглашение (раскрывает контакты работодателю)' })
  accept(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.invitations.respond(user.id, id, { status: 'ACCEPTED' });
  }

  @Post(':id/reject')
  @HttpCode(200)
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Отклонить приглашение' })
  reject(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.invitations.respond(user.id, id, { status: 'REJECTED' });
  }

  @Post(':id/withdraw')
  @HttpCode(200)
  @Roles(UserRole.EMPLOYER)
  @ApiOperation({ summary: 'Отозвать приглашение' })
  withdraw(@CurrentUser() user: AuthUser, @Param('id') id: string) {
    return this.invitations.withdraw(user.id, id);
  }
}
