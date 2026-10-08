import { Grade, InvitationStatus, Specialization } from '@prisma/client';
import { makeRng, pick, sample } from './rng';
import { STACKS, CandidateSeed } from './people';
import { SPECIALIZATIONS, SPEC_LABELS } from './questions';

export interface EmployerSeed {
  email: string;
  companyName: string;
  companyDescription: string;
  industry: string;
  website: string;
  contactPerson: string;
  contactEmail: string;
  contactPhone: string;
  isDemo?: boolean;
}

export const EMPLOYERS: EmployerSeed[] = [
  {
    email: 'test@employer.com',
    companyName: 'ООО «ТехноСтар»',
    companyDescription: 'Финтех-команда, разрабатывающая платёжную платформу и личный кабинет бизнеса. 120 инженеров, продуктовый подход, акцент на качество и надёжность.',
    industry: 'Финтех',
    website: 'https://technostar.example.ru',
    contactPerson: 'Ольга Васильева',
    contactEmail: 'hr@technostar.example.ru',
    contactPhone: '+7 495 100-20-30',
    isDemo: true,
  },
  {
    email: 'hr@company.com',
    companyName: 'АО «Северный Кластер»',
    companyDescription: 'Облачный провайдер: управляемые кластеры, мониторинг и CI/CD для продуктовых команд.',
    industry: 'Облачные сервисы',
    website: 'https://northcluster.example.ru',
    contactPerson: 'Игорь Лебедев',
    contactEmail: 'jobs@northcluster.example.ru',
    contactPhone: '+7 812 200-30-40',
    isDemo: true,
  },
  {
    email: 'hr3@example.com',
    companyName: 'ООО «ЛогикаПлюс»',
    companyDescription: 'Ритейл-технологии: система управления ассортиментом, мобильные приложения и аналитика продаж.',
    industry: 'Ритейл',
    website: 'https://logicaplus.example.ru',
    contactPerson: 'Наталья Кузнецова',
    contactEmail: 'career@logicaplus.example.ru',
    contactPhone: '+7 495 310-40-50',
  },
  {
    email: 'hr4@example.com',
    companyName: 'ООО «МедТех Диджитал»',
    companyDescription: 'Цифровые сервисы для клиник: электронная карта пациента, запись на приём, телемедицина.',
    industry: 'Здравоохранение',
    website: 'https://medtechd.example.ru',
    contactPerson: 'Алексей Морозов',
    contactEmail: 'hr@medtechd.example.ru',
    contactPhone: '+7 495 410-50-60',
  },
  {
    email: 'hr5@example.com',
    companyName: 'ГК «Цифровой Транспорт»',
    companyDescription: 'Логистическая платформа: маршрутизация, телеметрия транспорта и диспетчерская система.',
    industry: 'Логистика',
    website: 'https://dtrans.example.ru',
    contactPerson: 'Марина Орлова',
    contactEmail: 'team@dtrans.example.ru',
    contactPhone: '+7 495 510-60-70',
  },
  {
    email: 'hr6@example.com',
    companyName: 'ООО «НейроСофт»',
    companyDescription: 'ИИ-продукты для бизнеса: интеллектуальный поиск, ассистенты на LLM, аналитика документов.',
    industry: 'ИИ и ML',
    website: 'https://neurosoft.example.ru',
    contactPerson: 'Дмитрий Захаров',
    contactEmail: 'jobs@neurosoft.example.ru',
    contactPhone: '+7 495 610-70-80',
  },
  {
    email: 'hr7@example.com',
    companyName: 'ООО «Игровая Мастерская»',
    companyDescription: 'Инди-студия: мобильные и PC-игры на Unity, собственный сетевой фреймворк.',
    industry: 'Игры',
    website: 'https://gamemaster.example.ru',
    contactPerson: 'Артём Беляев',
    contactEmail: 'dev@gamemaster.example.ru',
    contactPhone: '+7 495 710-80-90',
  },
  {
    email: 'hr8@example.com',
    companyName: 'ООО «Безопасный Контур»',
    companyDescription: 'Кибербезопасность: SOC, пентесты, аудит приложений и защита персональных данных.',
    industry: 'Кибербезопасность',
    website: 'https://securecontour.example.ru',
    contactPerson: 'Сергей Никитин',
    contactEmail: 'hr@securecontour.example.ru',
    contactPhone: '+7 495 810-90-10',
  },
];

const SPEC_TITLES: Record<string, string> = {
  BACKEND: 'Backend-разработчик',
  FRONTEND: 'Frontend-разработчик',
  FULLSTACK: 'Fullstack-разработчик',
  MOBILE: 'Mobile-разработчик',
  DEVOPS: 'DevOps-инженер',
  DATA_SCIENCE: 'Data Scientist',
  QA: 'QA-инженер',
  SECURITY: 'Security-инженер',
  EMBEDDED: 'Embedded-разработчик',
  GAME_DEV: 'Gameplay-программист',
  ML_AI: 'ML-инженер',
  DESIGN_UX: 'UI/UX-дизайнер',
  MANAGEMENT: 'Project Manager',
};

const SALARY: Record<Grade, [number, number]> = {
  [Grade.JUNIOR]: [120000, 180000],
  [Grade.MIDDLE]: [220000, 320000],
  [Grade.MIDDLE_PLUS]: [300000, 400000],
  [Grade.SENIOR]: [380000, 520000],
  [Grade.LEAD]: [480000, 650000],
  [Grade.PRINCIPAL]: [600000, 900000],
};

const GRADE_CYCLE: Grade[] = [
  Grade.MIDDLE,
  Grade.MIDDLE,
  Grade.JUNIOR,
  Grade.MIDDLE_PLUS,
  Grade.SENIOR,
  Grade.MIDDLE,
  Grade.LEAD,
  Grade.MIDDLE_PLUS,
  Grade.SENIOR,
  Grade.JUNIOR,
  Grade.MIDDLE,
];

export interface VacancySeed {
  employerIndex: number;
  title: string;
  description: string;
  specialization: Specialization;
  grade: Grade;
  stack: string[];
  salaryFrom: number;
  salaryTo: number;
  isActive: boolean;
}

/** 30 вакансий на 8 работодателей (демо-работодатель — сильный backend-профиль). */
export function buildVacancies(): VacancySeed[] {
  const rng = makeRng(424242);
  const vacancies: VacancySeed[] = [];

  for (let i = 0; i < 30; i++) {
    const spec = i === 0 || i === 1 ? Specialization.BACKEND : SPECIALIZATIONS[i % SPECIALIZATIONS.length];
    const grade = GRADE_CYCLE[i % GRADE_CYCLE.length];
    const stack = sample(rng, STACKS[spec], 5);
    const [salaryFrom, salaryTo] = SALARY[grade];
    const employerIndex = i % EMPLOYERS.length;
    const title = `${SPEC_TITLES[spec]} (${stack[0]})`;
    const specLabel = SPEC_LABELS[spec];

    vacancies.push({
      employerIndex,
      title,
      description: `Ищем специалиста: ${title.toLowerCase()} в продуктовую команду «${EMPLOYERS[employerIndex].companyName}». Направление: ${specLabel}. Стек: ${stack.join(', ')}. Задачи: разработка функциональности, участие в архитектурных решениях, code review и развитие продукта. Требования: уровень ${grade}, уверенная работа с ${stack[0]} и ${stack[1]}, понимание принципов ${spec === Specialization.DEVOPS ? 'надёжности и наблюдаемости' : 'чистого кода и тестирования'}.`,
      specialization: spec,
      grade,
      stack,
      salaryFrom: salaryFrom + (i % 5) * 10000,
      salaryTo: salaryTo + (i % 4) * 15000,
      isActive: i % 9 !== 8,
    });
  }

  return vacancies;
}

export interface InvitationSeed {
  employerIndex: number;
  candidateEmail: string;
  vacancyIndex: number | null;
  message: string;
  salaryFrom: number;
  salaryTo: number;
  status: InvitationStatus;
}

/** 15 приглашений, включая демо-кандидатов (EMPLOYER-first модель). */
export function buildInvitations(candidates: CandidateSeed[]): InvitationSeed[] {
  const rng = makeRng(151515);
  const invitations: InvitationSeed[] = [];
  const [min, max] = SALARY[Grade.MIDDLE];

  invitations.push(
    {
      employerIndex: 0,
      candidateEmail: 'test@candidate.com',
      vacancyIndex: 0,
      message: 'Иван, приглашаем в команду бэкенда: интересные задачи по платёжной платформе и highload.',
      salaryFrom: 260000,
      salaryTo: 340000,
      status: InvitationStatus.ACCEPTED,
    },
    {
      employerIndex: 1,
      candidateEmail: 'test@candidate.com',
      vacancyIndex: null,
      message: 'Вакансия backend-разработчика в облачной платформе — посмотрите, пожалуйста.',
      salaryFrom: 280000,
      salaryTo: 360000,
      status: InvitationStatus.SENT,
    },
    {
      employerIndex: 2,
      candidateEmail: 'demo2@candidate.com',
      vacancyIndex: 2,
      message: 'Дарья, ищем сильного фронтенда для редизайна каталога.',
      salaryFrom: 400000,
      salaryTo: 520000,
      status: InvitationStatus.VIEWED,
    },
    {
      employerIndex: 1,
      candidateEmail: 'demo3@candidate.com',
      vacancyIndex: 3,
      message: 'Максим, приглашаем в инфраструктурную команду: Kubernetes и IaC.',
      salaryFrom: 340000,
      salaryTo: 440000,
      status: InvitationStatus.ACCEPTED,
    },
    {
      employerIndex: 5,
      candidateEmail: 'demo2@candidate.com',
      vacancyIndex: null,
      message: 'Интересен ваш опыт UI — есть задача по дизайн-системе.',
      salaryFrom: 380000,
      salaryTo: 470000,
      status: InvitationStatus.REJECTED,
    },
  );

  const pool = candidates.filter((c) => !c.isDemo);
  const statuses: InvitationStatus[] = [
    InvitationStatus.SENT,
    InvitationStatus.SENT,
    InvitationStatus.SENT,
    InvitationStatus.VIEWED,
    InvitationStatus.VIEWED,
    InvitationStatus.ACCEPTED,
    InvitationStatus.ACCEPTED,
    InvitationStatus.REJECTED,
    InvitationStatus.WITHDRAWN,
    InvitationStatus.SENT,
  ];

  const usedPairs = new Set(invitations.map((i) => `${i.employerIndex}:${i.candidateEmail}`));
  let attempts = 0;
  let index = 0;
  while (index < statuses.length && attempts < 200) {
    attempts++;
    const candidate = pick(rng, pool);
    const employerIndex = int0(rng, EMPLOYERS.length);
    const key = `${employerIndex}:${candidate.email}`;
    if (usedPairs.has(key)) continue;
    usedPairs.add(key);

    const grade = candidate.verifiedGrade || candidate.declaredGrade;
    const [salaryFrom, salaryTo] = SALARY[grade];
    invitations.push({
      employerIndex,
      candidateEmail: candidate.email,
      vacancyIndex: rng() < 0.6 ? int0(rng, 30) : null,
      message: `Здравствуйте, ${candidate.firstName}! Вакансия ${SPEC_LABELS[candidate.specialization].toLowerCase()} — обсудим?`,
      salaryFrom: salaryFrom + min * 0 + 20000,
      salaryTo: salaryTo + 40000,
      status: statuses[index],
    });
    index++;
  }

  return invitations;
}

export interface NeedSeed {
  employerIndex: number;
  description: string;
  specialization?: Specialization;
  grade?: Grade;
  stack: string[];
}

/** Потребности работодателей — входные данные matching-движка. */
export function buildNeeds(): NeedSeed[] {
  return [
    {
      employerIndex: 0,
      description:
        'Ищем backend-разработчика на Node.js с опытом от 3 лет для платёжной платформы: REST API, PostgreSQL, Redis, очереди, высокая нагрузка.',
      specialization: Specialization.BACKEND,
      grade: Grade.MIDDLE,
      stack: ['Node.js', 'PostgreSQL', 'Redis', 'Docker'],
    },
    {
      employerIndex: 1,
      description:
        'Нужен DevOps-инженер с Kubernetes и Terraform для поддержки облачной платформы и пайплайнов CI/CD.',
      specialization: Specialization.DEVOPS,
      grade: Grade.MIDDLE_PLUS,
      stack: ['Kubernetes', 'Terraform', 'Docker', 'Prometheus'],
    },
    {
      employerIndex: 2,
      description: 'Frontend-разработчик на React и TypeScript для каталога и личного кабинета, важен опыт с дизайн-системой.',
      specialization: Specialization.FRONTEND,
      grade: Grade.SENIOR,
      stack: ['React', 'TypeScript', 'CSS'],
    },
    {
      employerIndex: 5,
      description: 'ML-инженер для продукта на LLM: RAG, векторный поиск, PyTorch, опыт деплоя моделей в прод.',
      specialization: Specialization.ML_AI,
      grade: Grade.SENIOR,
      stack: ['PyTorch', 'LLM', 'RAG', 'MLflow'],
    },
    {
      employerIndex: 3,
      description: 'QA-инженер с автоматизацией: Playwright и API-тестирование для медицинского сервиса.',
      specialization: Specialization.QA,
      grade: Grade.MIDDLE,
      stack: ['Playwright', 'API testing', 'pytest'],
    },
    {
      employerIndex: 4,
      description: 'Data Scientist для прогнозирования спроса: Python, SQL, Spark, аналитика и модели прогноза.',
      specialization: Specialization.DATA_SCIENCE,
      grade: Grade.MIDDLE_PLUS,
      stack: ['Python', 'SQL', 'Spark'],
    },
  ];
}

function int0(rng: () => number, max: number): number {
  return Math.floor(rng() * max);
}
