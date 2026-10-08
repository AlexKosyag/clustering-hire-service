import 'reflect-metadata';
import { ValidationPipe, Logger } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import { AppModule } from './app.module';
import { HttpExceptionFilter } from './common/filters/http-exception.filter';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  app.setGlobalPrefix('api');

  const frontendUrl = process.env.FRONTEND_URL || 'http://localhost:8080';
  app.enableCors({
    origin: [frontendUrl, 'http://localhost:8080', 'http://localhost:3000', 'http://127.0.0.1:8080'],
    credentials: true,
  });

  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      transform: true,
      transformOptions: { enableImplicitConversion: true },
    }),
  );
  app.useGlobalFilters(new HttpExceptionFilter());

  const swaggerConfig = new DocumentBuilder()
    .setTitle('FSP Platform API')
    .setDescription('Цифровая платформа-агрегатор ИТ-вакансий ФСП')
    .setVersion('1.0')
    .addBearerAuth({ type: 'http', scheme: 'bearer', bearerFormat: 'JWT' }, 'bearer')
    .build();
  const document = SwaggerModule.createDocument(app, swaggerConfig);
  SwaggerModule.setup('api/docs', app, document, { customSiteTitle: 'FSP API Docs' });

  const port = Number(process.env.PORT || 3000);
  await app.listen(port);
  new Logger('Bootstrap').log(`API: http://localhost:${port}/api | Swagger: http://localhost:${port}/api/docs`);
}

bootstrap();
