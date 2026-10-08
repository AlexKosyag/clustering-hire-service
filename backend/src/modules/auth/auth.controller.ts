import { Body, Controller, Get, HttpCode, Post, Req, Res } from '@nestjs/common';
import { ApiBearerAuth, ApiOperation, ApiTags } from '@nestjs/swagger';
import { Request, Response } from 'express';
import { AuthUser, CurrentUser, Public } from '../../common/decorators/auth.decorator';
import { parseCookies } from '../../common/utils/cookie.util';
import { AuthService } from './auth.service';
import { LoginDto, RegisterDto } from './dto/auth.dto';

const REFRESH_COOKIE = 'refreshToken';
const REFRESH_MAX_AGE = 7 * 24 * 60 * 60;

@ApiTags('Auth')
@Controller('auth')
export class AuthController {
  constructor(private readonly auth: AuthService) {}

  @Public()
  @Post('register')
  @ApiOperation({ summary: 'Регистрация (CANDIDATE или EMPLOYER)' })
  async register(@Body() dto: RegisterDto, @Res({ passthrough: true }) res: Response) {
    const result = await this.auth.register(dto);
    res.cookie(REFRESH_COOKIE, result.refreshToken, {
      httpOnly: true,
      sameSite: 'lax',
      maxAge: REFRESH_MAX_AGE,
      secure: process.env.NODE_ENV === 'production',
    });
    return result;
  }

  @Public()
  @Post('login')
  @HttpCode(200)
  @ApiOperation({ summary: 'Вход по email и паролю' })
  async login(@Body() dto: LoginDto, @Res({ passthrough: true }) res: Response) {
    const result = await this.auth.login(dto);
    res.cookie(REFRESH_COOKIE, result.refreshToken, {
      httpOnly: true,
      sameSite: 'lax',
      maxAge: REFRESH_MAX_AGE,
      secure: process.env.NODE_ENV === 'production',
    });
    return result;
  }

  @Public()
  @Post('refresh')
  @HttpCode(200)
  @ApiOperation({ summary: 'Обновление access token по refresh token из cookie' })
  async refresh(
    @Body() body: { refreshToken?: string },
    @Req() req: Request,
    @Res({ passthrough: true }) res: Response,
  ) {
    const fromCookie = parseCookies(req.headers.cookie)[REFRESH_COOKIE];
    const refreshToken = body?.refreshToken || fromCookie;
    const tokens = await this.auth.refresh(refreshToken || '');
    res.cookie(REFRESH_COOKIE, tokens.refreshToken, {
      httpOnly: true,
      sameSite: 'lax',
      maxAge: REFRESH_MAX_AGE,
      secure: process.env.NODE_ENV === 'production',
    });
    return tokens;
  }

  @Post('logout')
  @HttpCode(200)
  @ApiBearerAuth('bearer')
  @ApiOperation({ summary: 'Выход: инвалидация refresh token' })
  async logout(@CurrentUser() user: AuthUser, @Res({ passthrough: true }) res: Response) {
    await this.auth.logout(user.id);
    res.clearCookie(REFRESH_COOKIE, { path: '/' });
    return { message: 'Выход выполнен' };
  }

  @Get('me')
  @ApiBearerAuth('bearer')
  @ApiOperation({ summary: 'Текущий пользователь с профилем' })
  me(@CurrentUser() user: AuthUser) {
    return this.auth.me(user.id);
  }
}
