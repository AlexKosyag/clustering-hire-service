/**
 * Seed синтетических данных платформы ФСП.
 * Запуск: npm run db:seed (prisma db seed)
 *
 * Объёмы: 13 категорий × 66 вопросов (~858), 204 кандидата, 8 работодателей,
 * 30 вакансий, 15 приглашений, 6 потребностей, справочник ФСП.
 */
import { PrismaClient, TestStatus, SessionStatus, Grade } from '@prisma/client';
import * as bcrypt from 'bcryptjs';
import { FSP_ACHIEVEMENTS } from './seed/achievements';
import { buildQuestionSeeds, countsByCategory } from './seed/questions';
import { buildCandidates, CandidateSeed } from './seed/people';
import {
  EMPLOYERS,
  buildInvitations,
  buildNeeds,
  buildVacancies,
} from './seed/companies';

const prisma = new PrismaClient();
const DAY = 86_400_000;

async function cleanup() {
  await prisma.testAnswer.deleteMany();
  await prisma.testSession.deleteMany();
  await prisma.testQuestion.deleteMany();
  await prisma.testCategory.deleteMany();
  await prisma.invitation.deleteMany();
  await prisma.employerNeed.deleteMany();
  await prisma.vacancy.deleteMany();
  await prisma.candidateAchievement.deleteMany();
  await prisma.dailyTask.deleteMany();
  await prisma.candidateProfile.deleteMany();
  await prisma.employerProfile.deleteMany();
  await prisma.fSPAchievement.deleteMany();
  await prisma.user.deleteMany();
}

async function seedAchievements() {
  await prisma.fSPAchievement.createMany({ data: FSP_ACHIEVEMENTS, skipDuplicates: true });
}

async function seedQuestions() {
  const { categories, questions } = buildQuestionSeeds();
  const counts = countsByCategory(questions);

  await prisma.testCategory.createMany({
    data: categories.map((c) => ({
      name: c.name,
      description: c.description,
      specialization: c.specialization,
    })),
    skipDuplicates: true,
  });

  const rows = await prisma.testCategory.findMany({ select: { id: true, name: true, specialization: true } });
  const idByKey = new Map<string, string>();
  for (const category of rows) {
    idByKey.set(category.specialization, category.id);
  }

  const data = questions.map((q) => ({
    categoryId: idByKey.get(q.key)!,
    questionType: q.questionType,
    text: q.text,
    codeSnippet: q.codeSnippet ?? null,
    options: (q.options ?? null) as any,
    correctAnswer: q.correctAnswer ?? null,
    explanation: q.explanation,
    difficulty: q.difficulty,
    discrimination: q.discrimination,
    guessing: q.guessing,
    isUnique: q.isUnique,
    poolTag: q.poolTag,
  }));

  for (let i = 0; i < data.length; i += 200) {
    await prisma.testQuestion.createMany({ data: data.slice(i, i + 200), skipDuplicates: true });
  }

  return { total: data.length, counts };
}

async function seedCandidates(passwordHash: string, candidates: CandidateSeed[]) {
  const achievementRows = await prisma.fSPAchievement.findMany({ select: { id: true, name: true } });
  const achievementIdByName = new Map(achievementRows.map((a) => [a.name, a.id]));

  const userIdByEmail = new Map<string, string>();
  const created: Array<{ seed: CandidateSeed; profile: { id: string; specialization: any } }> = [];

  for (const candidate of candidates) {
    const user = await prisma.user.create({
      data: {
        email: candidate.email,
        passwordHash,
        role: 'CANDIDATE',
        isEmailVerified: true,
        candidateProfile: {
          create: {
            firstName: candidate.firstName,
            lastName: candidate.lastName,
            middleName: candidate.middleName,
            phone: candidate.phone,
            city: candidate.city,
            bio: candidate.bio,
            specialization: candidate.specialization,
            declaredGrade: candidate.declaredGrade,
            verifiedGrade: candidate.verifiedGrade,
            stack: candidate.stack,
            roles: candidate.roles,
            softSkills: candidate.softSkills,
            experienceYears: candidate.experienceYears,
            showEmail: candidate.showEmail,
            showPhone: candidate.showPhone,
            showFullName: candidate.showFullName,
            consentToProcessing: candidate.consentToProcessing,
            testStatus: candidate.testStatus,
            lastTestDate: candidate.lastTestDate,
            testCooldownUntil: candidate.testCooldownUntil,
            lastTestScore: candidate.lastTestScore,
            fspId: candidate.fspId,
            fspConnectedAt: candidate.fspId ? new Date(Date.now() - 60 * DAY) : null,
          },
        },
      },
      include: { candidateProfile: true },
    });
    userIdByEmail.set(candidate.email, user.id);
    created.push({ seed: candidate, profile: user.candidateProfile! });
  }

  // Связка достижений ФСП
  const links: Array<{ candidateId: string; achievementId: string }> = [];
  for (const item of created) {
    for (const name of item.seed.achievementNames) {
      const achievementId = achievementIdByName.get(name);
      if (!achievementId) continue;
      links.push({ candidateId: item.profile.id, achievementId });
    }
  }
  for (let i = 0; i < links.length; i += 500) {
    await prisma.candidateAchievement.createMany({ data: links.slice(i, i + 500), skipDuplicates: true });
  }

  // История тестирований (без ответов — реальные сессии создаёт движок)
  const tested = created.filter((c) => c.seed.testStatus !== TestStatus.NOT_STARTED && c.seed.lastTestDate);
  for (const item of tested) {
    const started = item.seed.lastTestDate!;
    await prisma.testSession.create({
      data: {
        candidateId: item.profile.id,
        categoryId: (await categoryIdFor(item.seed.specialization))!,
        status: SessionStatus.COMPLETED,
        startedAt: new Date(started.getTime() - 45 * 60_000),
        completedAt: started,
        score: item.seed.lastTestScore ?? 0,
        rawScore: item.seed.lastTestScore ?? 0,
        maxScore: 100,
        gradeBefore: item.seed.declaredGrade,
        gradeAfter: item.seed.verifiedGrade ?? item.seed.declaredGrade,
        recommendation:
          item.seed.testStatus === TestStatus.FAILED
            ? 'FAIL'
            : item.seed.testStatus === TestStatus.UPGRADED
              ? 'UPGRADE'
              : 'PASS',
        thetaEstimate: thetaFor(item.seed.verifiedGrade ?? item.seed.declaredGrade),
      },
    });
  }

  return { total: created.length, achievementLinks: links.length, testedSessions: tested.length, userIdByEmail, created };
}

const categoryCache = new Map<string, string | null>();
async function categoryIdFor(specialization: string): Promise<string | null> {
  if (categoryCache.has(specialization)) return categoryCache.get(specialization)!;
  const category = await prisma.testCategory.findFirst({ where: { specialization: specialization as any } });
  categoryCache.set(specialization, category?.id ?? null);
  return category?.id ?? null;
}

function thetaFor(grade: Grade): number {
  const map: Record<string, number> = {
    JUNIOR: -1.6,
    MIDDLE: -0.4,
    MIDDLE_PLUS: 0.3,
    SENIOR: 0.8,
    LEAD: 1.3,
    PRINCIPAL: 1.8,
  };
  return map[grade] ?? 0;
}

async function seedEmployers(passwordHash: string) {
  const ids: string[] = [];
  for (const employer of EMPLOYERS) {
    const user = await prisma.user.create({
      data: {
        email: employer.email,
        passwordHash,
        role: 'EMPLOYER',
        isEmailVerified: true,
        employerProfile: {
          create: {
            companyName: employer.companyName,
            companyDescription: employer.companyDescription,
            industry: employer.industry,
            website: employer.website,
            contactPerson: employer.contactPerson,
            contactEmail: employer.contactEmail,
            contactPhone: employer.contactPhone,
          },
        },
      },
      include: { employerProfile: true },
    });
    ids.push(user.employerProfile!.id);
  }
  return ids;
}

async function main() {
  console.log('▶ Очистка данных...');
  await cleanup();

  console.log('▶ Справочник достижений ФСП...');
  await seedAchievements();

  console.log('▶ Категории и вопросы...');
  const questions = await seedQuestions();
  console.log(`   категорий: 13, вопросов: ${questions.total}`);
  console.log('   по категориям:', JSON.stringify(questions.counts));

  console.log('▶ Демо-пароль и кандидаты...');
  const passwordHash = await bcrypt.hash('Test123!', 10);
  const candidateSeeds = buildCandidates(204);
  const candidates = await seedCandidates(passwordHash, candidateSeeds);
  console.log(`   кандидатов: ${candidates.total}, связок достижений: ${candidates.achievementLinks}, сессий тестов: ${candidates.testedSessions}`);

  console.log('▶ Работодатели...');
  const employerIds = await seedEmployers(passwordHash);
  console.log(`   работодателей: ${employerIds.length}`);

  console.log('▶ Вакансии...');
  const vacancySeeds = buildVacancies();
  const vacancyIds: string[] = [];
  for (const vacancy of vacancySeeds) {
    const created = await prisma.vacancy.create({
      data: {
        employerId: employerIds[vacancy.employerIndex],
        title: vacancy.title,
        description: vacancy.description,
        specialization: vacancy.specialization,
        grade: vacancy.grade,
        stack: vacancy.stack,
        salaryFrom: vacancy.salaryFrom,
        salaryTo: vacancy.salaryTo,
        isActive: vacancy.isActive,
      },
    });
    vacancyIds.push(created.id);
  }
  console.log(`   вакансий: ${vacancyIds.length}`);

  console.log('▶ Приглашения...');
  const invitationSeeds = buildInvitations(candidateSeeds);
  let invitations = 0;
  for (const invitation of invitationSeeds) {
    const candidate = await prisma.candidateProfile.findFirst({
      where: { user: { email: invitation.candidateEmail } },
      select: { id: true },
    });
    if (!candidate) continue;
    await prisma.invitation.create({
      data: {
        employerId: employerIds[invitation.employerIndex],
        candidateId: candidate.id,
        vacancyId: invitation.vacancyIndex != null ? vacancyIds[invitation.vacancyIndex] : null,
        message: invitation.message,
        salaryFrom: invitation.salaryFrom,
        salaryTo: invitation.salaryTo,
        status: invitation.status,
        statusUpdatedAt: new Date(Date.now() - Math.floor(Math.random() * 7) * DAY),
        createdAt: new Date(Date.now() - Math.floor(Math.random() * 20) * DAY),
      },
    });
    invitations++;
  }
  console.log(`   приглашений: ${invitations}`);

  console.log('▶ Потребности работодателей (вход matching)...');
  let needs = 0;
  for (const need of buildNeeds()) {
    await prisma.employerNeed.create({
      data: {
        employerId: employerIds[need.employerIndex],
        description: need.description,
        specialization: need.specialization,
        grade: need.grade,
        stack: need.stack,
        createdAt: new Date(Date.now() - Math.floor(Math.random() * 10) * DAY),
      },
    });
    needs++;
  }
  console.log(`   потребностей: ${needs}`);

  console.log('');
  console.log('✅ Seed завершён.');
  console.log('   Демо-аккаунты (пароль Test123!):');
  console.log('   - test@candidate.com  (кандидат, Backend/Middle)');
  console.log('   - demo2@candidate.com (кандидат, Frontend/Senior)');
  console.log('   - demo3@candidate.com (кандидат, DevOps/Middle+)');
  console.log('   - test@employer.com   (работодатель, ООО «ТехноСтар»)');
  console.log('   - hr@company.com      (работодатель, АО «Северный Кластер»)');
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(async () => {
    await prisma.$disconnect();
  });
