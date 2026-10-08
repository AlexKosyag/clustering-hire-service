/**
 * E2E-тесты API (реальное приложение + реальная БД из docker-compose).
 *   npm run test:e2e
 *
 * Каждый прогон регистрирует свежие аккаунты — тест идемпотентен и не портит
 * демо-данные сидера.
 */
import 'reflect-metadata';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import { AppModule } from '../src/app.module';
import { HttpExceptionFilter } from '../src/common/filters/http-exception.filter';

jest.setTimeout(180000);

interface Tokens {
  accessToken: string;
  refreshToken: string;
  user: { id: string; email: string; role: string };
}

describe('FSP Platform API (e2e)', () => {
  let app: INestApplication;
  let server: any;

  let candidate: Tokens;
  let employer: Tokens;
  let candidateProfileId: string;
  let invitationId: string;

  const stamp = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  const candidateEmail = `e2e-candidate-${stamp}@example.com`;
  const employerEmail = `e2e-employer-${stamp}@example.com`;

  const http = () => request(server);

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
    app = moduleRef.createNestApplication();
    app.setGlobalPrefix('api');
    app.useGlobalPipes(
      new ValidationPipe({
        whitelist: true,
        transform: true,
        transformOptions: { enableImplicitConversion: true },
      }),
    );
    app.useGlobalFilters(new HttpExceptionFilter());

    // Swagger, как в src/main.ts — e2e дополнительно проверяет публикацию API-документации
    const swaggerConfig = new DocumentBuilder()
      .setTitle('FSP Platform API')
      .setVersion('1.0')
      .addBearerAuth({ type: 'http', scheme: 'bearer', bearerFormat: 'JWT' }, 'bearer')
      .build();
    SwaggerModule.setup('api/docs', app, SwaggerModule.createDocument(app, swaggerConfig));

    await app.init();
    server = app.getHttpServer();
  });

  afterAll(async () => {
    await app?.close();
  });

  describe('Health и доступность', () => {
    it('GET /api/health — ok, pgvector-размерность 384', async () => {
      const res = await http().get('/api/health').expect(200);
      expect(res.body.status).toBe('ok');
      expect(res.body.services.database).toBe('up');
      expect(res.body.services.embedding.dim).toBe(384);
    });

    it('незащищённые маршруты доступны без токена', async () => {
      await http().get('/api/auth/me').expect(401);
      await http().get('/api/candidates/profile').expect(401);
    });

    it('некорректный JSON/payload → 4xx, а не 500', async () => {
      const res = await http()
        .post('/api/auth/register')
        .send({ email: 'not-an-email', password: '1', role: 'CANDIDATE' })
        .expect(400);
      expect(res.body.message).toBeDefined();
    });
  });

  describe('Auth: регистрация, вход, роли', () => {
    it('регистрирует кандидата и выдаёт токены', async () => {
      const res = await http()
        .post('/api/auth/register')
        .send({ email: candidateEmail, password: 'E2eTest1234!', role: 'CANDIDATE' })
        .expect(201);
      candidate = res.body;
      expect(candidate.accessToken).toBeTruthy();
      expect(candidate.refreshToken).toBeTruthy();
      expect(candidate.user.role).toBe('CANDIDATE');
    });

    it('дубликат email → 409', async () => {
      await http()
        .post('/api/auth/register')
        .send({ email: candidateEmail, password: 'E2eTest1234!', role: 'CANDIDATE' })
        .expect(409);
    });

    it('вход по паролю возвращает тот же профиль', async () => {
      const res = await http()
        .post('/api/auth/login')
        .send({ email: candidateEmail, password: 'E2eTest1234!' })
        .expect(200);
      expect(res.body.user.email).toBe(candidateEmail);
      expect(res.body.accessToken).toBeTruthy();
    });

    it('неверный пароль → отказ', async () => {
      const res = await http()
        .post('/api/auth/login')
        .send({ email: candidateEmail, password: 'WrongPassword1!' });
      expect(res.status).toBeGreaterThanOrEqual(400);
      expect(res.status).toBeLessThan(500);
    });

    it('refresh выдаёт новую пару токенов', async () => {
      const res = await http()
        .post('/api/auth/refresh')
        .send({ refreshToken: candidate.refreshToken })
        .expect(200);
      expect(res.body.accessToken).toBeTruthy();
      candidate.accessToken = res.body.accessToken;
    });

    it('GET /api/auth/me возвращает пользователя', async () => {
      const res = await http()
        .get('/api/auth/me')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      expect(res.body.email).toBe(candidateEmail);
    });
  });

  describe('Кандидат: профиль', () => {
    it('профиль создаётся автоматически при регистрации', async () => {
      const res = await http()
        .get('/api/candidates/profile')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      candidateProfileId = res.body.id;
      expect(candidateProfileId).toBeTruthy();
      expect(res.body).toHaveProperty('declaredGrade');
      expect(Array.isArray(res.body.stack)).toBe(true);
    });

    it('обновление профиля сохраняет стек и специализацию', async () => {
      const res = await http()
        .put('/api/candidates/profile')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({
          firstName: 'E2E',
          lastName: 'Tester',
          specialization: 'BACKEND',
          declaredGrade: 'MIDDLE',
          stack: ['Node.js', 'PostgreSQL', 'Docker'],
          city: 'Москва',
          bio: 'E2E-кандидат для проверки API',
        })
        .expect(200);
      expect(res.body.stack).toEqual(['Node.js', 'PostgreSQL', 'Docker']);
      expect(res.body.specialization).toBe('BACKEND');
    });
  });

  describe('Разграничение прав (roles)', () => {
    it('работодатель не берёт профиль кандидата', async () => {
      const login = await http()
        .post('/api/auth/login')
        .send({ email: 'test@employer.com', password: 'Test123!' })
        .expect(200);
      employer = login.body;
      expect(employer.user.role).toBe('EMPLOYER');

      await http()
        .get('/api/candidates/profile')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .expect(403);
    });

    it('кандидат не берёт работодательские маршруты', async () => {
      await http()
        .get('/api/employers/dashboard')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(403);
    });
  });

  describe('Matching Engine', () => {
    let searchMs = 0;

    it('поиск кандидатов: hard-фильтры, скоринг, объяснение', async () => {
      const started = Date.now();
      const res = await http()
        .post('/api/matching/search')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .send({
          description:
            'Ищем backend-разработчика на Node.js: REST API, PostgreSQL, Redis, очереди, высокая нагрузка',
          specialization: 'BACKEND',
          grade: 'MIDDLE',
          stack: ['Node.js', 'PostgreSQL', 'Redis', 'Docker'],
          limit: 10,
        })
        .expect(201);
      searchMs = Date.now() - started;

      expect(res.body.total).toBeGreaterThan(0);
      expect(res.body.candidates.length).toBeGreaterThan(0);
      // метрика VALIDATION.md: поиск < 500 ms
      expect(searchMs).toBeLessThan(500);

      for (const item of res.body.candidates) {
        expect(item.specialization).toBe('BACKEND'); // hard-фильтр
        expect(item.matchScore).toBeGreaterThanOrEqual(0);
        expect(item.matchScore).toBeLessThanOrEqual(100);
        expect(Number.isFinite(item.matchScore)).toBe(true);
        expect(item.matchExplanation.details.length).toBeGreaterThan(0);
      }
      // результаты отсортированы по убыванию score
      const scores = res.body.candidates.map((c: { matchScore: number }) => c.matchScore);
      expect([...scores].sort((a: number, b: number) => b - a)).toEqual(scores);
    });

    it('semantic-поиск возвращает кандидатов с косинусной близостью', async () => {
      const res = await http()
        .post('/api/matching/semantic')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .send({ query: 'разработчик с опытом работы с Redis и очередями сообщений', limit: 5 })
        .expect(201);
      expect(res.body.candidates.length).toBeGreaterThan(0);
      const first = res.body.candidates[0];
      expect(first.similarity).toBeGreaterThanOrEqual(0);
      expect(first.similarity).toBeLessThanOrEqual(1);
      expect(typeof first.reason).toBe('string');
    });

    it('рекомендации вакансий для работодателя не падают', async () => {
      await http()
        .get('/api/matching/recommendations')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .expect(200);
    });

    it('поиск без указания специализации работает и не выдаёт NaN', async () => {
      const res = await http()
        .post('/api/matching/search')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .send({ description: 'нужен инженер для поддержки продакшена', limit: 5 })
        .expect(201);
      expect(res.body.candidates.every((c: { matchScore: number }) => Number.isFinite(c.matchScore))).toBe(true);
    });
  });

  describe('Testing Engine: полный цикл CAT-теста', () => {
    let sessionId: string;

    it('старт теста создаёт сессию с 20 вопросами', async () => {
      const res = await http()
        .post('/api/testing/start')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({ specialization: 'BACKEND', declaredGrade: 'MIDDLE' })
        .expect(201);
      sessionId = res.body.sessionId;
      expect(sessionId).toBeTruthy();
      expect(res.body.totalQuestions).toBe(20);
      expect(res.body.timeLimitMinutes).toBeGreaterThan(0);
    });

    it('текущий вопрос не утекает ответы (no correctAnswer/isCorrect/explanation)', async () => {
      const res = await http()
        .get(`/api/testing/session/${sessionId}/current`)
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      expect(res.body.question).toBeTruthy();
      expect(res.body.question.id).toBeTruthy();
      const raw = JSON.stringify(res.body);
      expect(raw).not.toMatch(/"correctAnswer"/);
      expect(raw).not.toMatch(/"isCorrect"/);
      expect(raw).not.toMatch(/"explanation"/);
      if (res.body.question.questionType === 'multiple_choice') {
        expect(res.body.question.options.length).toBeGreaterThanOrEqual(2);
      }
    });

    it('повторный старт при активной сессии → 409', async () => {
      await http()
        .post('/api/testing/start')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({ specialization: 'BACKEND' })
        .expect(409);
    });

    it('ответы принимаются и накапливаются, завершение выдаёт результат', async () => {
      let answered = 0;
      for (let i = 0; i < 20; i++) {
        const current = await http()
          .get(`/api/testing/session/${sessionId}/current`)
          .set('Authorization', `Bearer ${candidate.accessToken}`)
          .expect(200);
        const question = current.body.question;
        if (!question) break;

        const body: Record<string, unknown> = { questionId: question.id, timeSpentSeconds: 8 };
        if (question.questionType === 'multiple_choice') {
          body.answer = question.options[i % question.options.length].id;
        } else {
          body.answerText =
            'Использую SELECT JOIN транзакции валидацию тесты документацию try catch';
        }
        await http()
          .post(`/api/testing/session/${sessionId}/answer`)
          .set('Authorization', `Bearer ${candidate.accessToken}`)
          .send(body)
          .expect(201);
        answered++;
      }
      expect(answered).toBeGreaterThan(10);

      const res = await http()
        .post(`/api/testing/session/${sessionId}/complete`)
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({})
        .expect(201);
      expect(res.body.score).toBeGreaterThanOrEqual(0);
      expect(res.body.score).toBeLessThanOrEqual(100);
      expect(['PASS', 'FAIL', 'UPGRADE']).toContain(res.body.recommendation);
      expect(res.body.theta).toBeGreaterThanOrEqual(-4);
      expect(res.body.theta).toBeLessThanOrEqual(4);
      expect(res.body.antiCheat).toBeDefined();
      expect(res.body.cooldownUntil).toBeTruthy();
    });

    it('после теста повторный старт блокируется cooldown', async () => {
      const res = await http()
        .post('/api/testing/start')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({ specialization: 'BACKEND' });
      expect(res.status).toBe(400);
    });
  });

  describe('Вакансии', () => {
    it('список вакансий содержит данные сидера (30+)', async () => {
      const res = await http().get('/api/vacancies?limit=5&onlyActive=false').expect(200);
      expect(res.body.total).toBeGreaterThanOrEqual(30);
      expect(res.body.vacancies[0].title).toBeTruthy();
    });

    it('детальная вакансия возвращается по id', async () => {
      const list = await http().get('/api/vacancies?limit=1').expect(200);
      await http().get(`/api/vacancies/${list.body.vacancies[0].id}`).expect(200);
    });
  });

  describe('Приглашения и приватность контактов', () => {
    it('создание приглашения работодателем, контакты скрыты', async () => {
      const res = await http()
        .post('/api/invitations')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .send({
          candidateId: candidateProfileId,
          message: 'Приглашаем обсудить вакансию backend-разработчика',
          salaryFrom: 250000,
          salaryTo: 350000,
        })
        .expect(201);
      invitationId = res.body.id;
      expect(invitationId).toBeTruthy();

      const detail = await http()
        .get(`/api/invitations/${invitationId}`)
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .expect(200);
      expect(detail.body.candidate.contactsLocked).toBe(true);
      expect(detail.body.candidate.contacts).toBeNull();
    });

    it('поиск кандидатов не раскрывает email (privacy by design)', async () => {
      const res = await http()
        .get('/api/employers/candidates/search?specialization=BACKEND&limit=3')
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .expect(200);
      expect(res.body.candidates.length).toBeGreaterThan(0);
      expect(res.body.candidates[0].email).toBeUndefined();
    });

    it('принятие приглашения раскрывает контакты только приглашённому', async () => {
      const mine = await http()
        .get('/api/invitations')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      expect(mine.body.invitations.length).toBeGreaterThan(0);

      await http()
        .post(`/api/invitations/${invitationId}/accept`)
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .send({})
        .expect(200);

      const detail = await http()
        .get(`/api/invitations/${invitationId}`)
        .set('Authorization', `Bearer ${employer.accessToken}`)
        .expect(200);
      expect(detail.body.status).toBe('ACCEPTED');
      expect(detail.body.candidate.contacts.email).toBe(candidateEmail);
    });
  });

  describe('FSP mock, уведомления, документация', () => {
    it('FSP-каталог доступен', async () => {
      const res = await http()
        .get('/api/fsp/catalog')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      expect(res.body).toBeTruthy();
    });

    it('уведомления кандидата содержат записи', async () => {
      const res = await http()
        .get('/api/notifications')
        .set('Authorization', `Bearer ${candidate.accessToken}`)
        .expect(200);
      expect(res.body.total).toBeGreaterThan(0);
    });

    it('Swagger отдаётся по /api/docs', async () => {
      const res = await http().get('/api/docs-json').expect(200);
      expect(res.body.openapi).toBeDefined();
      expect(Object.keys(res.body.paths).length).toBeGreaterThan(20);
    });
  });
});
