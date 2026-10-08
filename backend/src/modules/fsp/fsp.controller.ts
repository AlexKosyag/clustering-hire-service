import { Body, Controller, Get, HttpCode, Post } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiProperty, ApiTags } from '@nestjs/swagger';
import { UserRole } from '@prisma/client';
import { IsString, Matches } from 'class-validator';
import { AuthUser, CurrentUser, Roles } from '../../common/decorators/auth.decorator';
import { FspService } from './fsp.service';

export class ConnectFspDto {
  @ApiProperty({ example: 'FSP-12345', description: 'Идентификатор в системе ФСП' })
  @IsString()
  @Matches(/^[A-Za-z0-9_-]{4,40}$/, { message: 'Некорректный формат FSP ID' })
  fspId: string;
}

@ApiTags('FSP')
@ApiBearerAuth('bearer')
@Controller('fsp')
export class FspController {
  constructor(private readonly fsp: FspService) {}

  @Post('connect')
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Подключить аккаунт ФСП (mock: возвращает достижения)' })
  connect(@CurrentUser() user: AuthUser, @Body() dto: ConnectFspDto) {
    return this.fsp.connect(user.id, dto.fspId);
  }

  @Post('disconnect')
  @HttpCode(200)
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Отключить аккаунт ФСП и удалить достижения' })
  disconnect(@CurrentUser() user: AuthUser) {
    return this.fsp.disconnect(user.id);
  }

  @Get('achievements')
  @Roles(UserRole.CANDIDATE)
  @ApiOperation({ summary: 'Достижения ФСП кандидата' })
  achievements(@CurrentUser() user: AuthUser) {
    return this.fsp.achievements(user.id);
  }

  @Get('catalog')
  @ApiOperation({ summary: 'Справочник достижений ФСП (mock-каталог из seed)' })
  catalog() {
    return this.fsp.catalog();
  }
}
