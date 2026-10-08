import { Grade, Specialization, TestStatus } from '@prisma/client';
import { SPECIALIZATIONS, SPEC_LABELS } from './questions';
import { int, makeRng, pick, sample } from './rng';

export const STACKS: Record<string, string[]> = {
  BACKEND: ['Node.js', 'TypeScript', 'PostgreSQL', 'Redis', 'Docker', 'Go', 'Kafka', 'gRPC'],
  FRONTEND: ['React', 'TypeScript', 'Next.js', 'CSS', 'GraphQL', 'Zustand', 'Vite', 'Storybook'],
  FULLSTACK: ['React', 'Node.js', 'TypeScript', 'PostgreSQL', 'Docker', 'Redis', 'REST API'],
  MOBILE: ['Swift', 'Kotlin', 'Flutter', 'React Native', 'Firebase', 'iOS', 'Android'],
  DEVOPS: ['Kubernetes', 'Docker', 'Terraform', 'Ansible', 'Prometheus', 'AWS', 'Helm', 'GitLab CI'],
  DATA_SCIENCE: ['Python', 'pandas', 'scikit-learn', 'SQL', 'Airflow', 'matplotlib', 'Spark'],
  QA: ['Selenium', 'Playwright', 'Postman', 'JMeter', 'pytest', 'Allure', 'API testing'],
  SECURITY: ['OWASP', 'Burp Suite', 'SIEM', 'Kali Linux', 'Snort', 'nist', 'pentest'],
  EMBEDDED: ['C', 'C++', 'FreeRTOS', 'STM32', 'I2C', 'SPI', 'RTOS', 'assembly'],
  GAME_DEV: ['Unity', 'C#', 'Unreal Engine', 'GLSL', 'Photon', 'gameplay', '3D math'],
  ML_AI: ['PyTorch', 'TensorFlow', 'Hugging Face', 'LangChain', 'MLflow', 'RAG', 'LLM'],
  DESIGN_UX: ['Figma', 'Design System', 'prototyping', 'usability testing', 'wireframes', 'a11y'],
  MANAGEMENT: ['Scrum', 'Agile', 'Jira', 'OKR', 'roadmap', 'stakeholders', 'backlog'],
};

const ROLES: Record<string, string[][]> = {
  BACKEND: [['Backend-разработчик'], ['Backend-разработчик', 'Тимлид'], ['Backend-разработчик', 'Архитектор']],
  FRONTEND: [['Frontend-разработчик'], ['Frontend-разработчик', 'Тимлид'], ['UI-инженер']],
  FULLSTACK: [['Fullstack-разработчик'], ['Fullstack-разработчик', 'Техлид']],
  MOBILE: [['Mobile-разработчик'], ['iOS-разработчик'], ['Android-разработчик']],
  DEVOPS: [['DevOps-инженер'], ['DevOps-инженер', 'SRE'], ['Инженер инфраструктуры']],
  DATA_SCIENCE: [['Data Scientist'], ['Data Analyst'], ['Data Scientist', 'Тимлид']],
  QA: [['QA-инженер'], ['QA-инженер', 'Автоматизатор'], ['QA Lead']],
  SECURITY: [['Security-инженер'], ['Application Security Engineer'], ['Аналитик SOC']],
  EMBEDDED: [['Embedded-разработчик'], ['Инженер прошивок'], ['Embedded-разработчик', 'Тимлид']],
  GAME_DEV: [['Gameplay-программист'], ['Графический программист'], ['Game-программист']],
  ML_AI: [['ML-инженер'], ['AI Researcher'], ['ML-инженер', 'Тимлид']],
  DESIGN_UX: [['UI/UX-дизайнер'], ['Продуктовый дизайнер'], ['UX-исследователь']],
  MANAGEMENT: [['Project Manager'], ['Engineering Manager'], ['Product Owner']],
};

const SOFT_SKILLS = [
  ['коммуникация', 'командная работа'],
  ['наставничество', 'приоритизация'],
  ['аналитическое мышление', 'ответственность'],
  ['организация', 'внимание к деталям'],
  ['лидерство', 'негибкое мышление'],
];

const SPEC_PROJECTS: Record<string, string[]> = {
  BACKEND: ['платёжного шлюза', 'сервиса уведомлений', 'API для мобильного клиента', 'системы очередей', 'высоконагруженного бэкенда'],
  FRONTEND: ['интерфейса CRM', 'онлайн-каталога', 'SPA-дашборда', 'конструктора отчётов', 'личного кабинета'],
  FULLSTACK: ['внутреннего портала', 'маркетплейса', 'сервиса подачи заявок', 'админ-панели', 'SaaS-продукта'],
  MOBILE: ['банковского приложения', 'фитнес-трекера', 'мессенджера', 'приложения доставки', 'образовательного приложения'],
  DEVOPS: ['Kubernetes-кластера', 'пайплайна CI/CD', 'мутооблачной инфраструктуры', 'системы мониторинга', 'платформы деплоя'],
  DATA_SCIENCE: ['рекомендательной системы', 'прогноза оттока', 'аналитики воронки', 'системы прогнозирования спроса', 'дашборда метрик'],
  QA: ['регрессионного набора автотестов', 'нагрузочного стенда', 'API-тестирования', 'тестовой документации', 'автоматизации релизов'],
  SECURITY: ['аудита веб-приложения', 'мониторинга SOC', 'программы пентестов', 'защиты персональных данных', 'системы обнаружения инцидентов'],
  EMBEDDED: ['контроллера умного дома', 'прошивки датчиков', 'протокола обмена с периферией', 'устройств сбора телеметрии', 'бортового компьютера'],
  GAME_DEV: ['индии-платформера', 'сетевого шутера', 'мобильной казуальной игры', 'системы генерации уровней', 'геймплея-прототипа'],
  ML_AI: ['классификатора обращений', 'поиска по документам', 'ассистента на LLM', 'детектора аномалий', 'системы векторного поиска'],
  DESIGN_UX: ['дизайн-системы', 'редизайна онбординга', 'мобильного интерфейса', 'исследования юзабилити', 'каталога компонентов'],
  MANAGEMENT: ['программы разработки', 'масштабирования команды', 'процесса планирования', 'системы целей', 'найм-воронки'],
};

const FIRST_NAMES_M = ['Иван', 'Алексей', 'Дмитрий', 'Сергей', 'Андрей', 'Михаил', 'Никита', 'Павел', 'Владимир', 'Кирилл', 'Егор', 'Тимур', 'Артём', 'Роман', 'Денис', 'Максим'];
const FIRST_NAMES_F = ['Анна', 'Мария', 'Екатерина', 'Ольга', 'Наталья', 'Елена', 'Дарья', 'Ксения', 'Полина', 'Виктория', 'Алиса', 'Софья', 'Ирина', 'Юлия', 'Алла', 'Людмила'];
const LAST_NAMES_M = ['Иванов', 'Петров', 'Смирнов', 'Кузнецов', 'Попов', 'Соколов', 'Лебедев', 'Козлов', 'Новиков', 'Морозов', 'Волков', 'Орлов', 'Макаров', 'Никитин', 'Захаров', 'Беляев'];
const LAST_NAMES_F = ['Иванова', 'Петрова', 'Смирнова', 'Кузнецова', 'Попова', 'Соколова', 'Лебедева', 'Козлова', 'Новикова', 'Морозова', 'Волкова', 'Орлова', 'Макарова', 'Никитина', 'Захарова', 'Беляева'];
const MIDDLE_M = ['Сергеевич', 'Александрович', 'Владимирович', 'Иванович', 'Дмитриевич', 'Павлович'];
const MIDDLE_F = ['Сергеевна', 'Александровна', 'Владимировна', 'Ивановна', 'Дмитриевна', 'Павловна'];
const CITIES = ['Москва', 'Санкт-Петербург', 'Казань', 'Новосибирск', 'Екатеринбург', 'Нижний Новгород', 'Самара', 'Владивосток', 'Минск', 'Алматы', 'Краснодар', 'Уфа'];

const GRADE_POOL: Grade[] = [
  ...Array(25).fill(Grade.JUNIOR),
  ...Array(34).fill(Grade.MIDDLE),
  ...Array(15).fill(Grade.MIDDLE_PLUS),
  ...Array(14).fill(Grade.SENIOR),
  ...Array(9).fill(Grade.LEAD),
  ...Array(3).fill(Grade.PRINCIPAL),
];

/** Полный разброс грейдов: первые кандидаты каждой специализации проходят по кругу. */
const ALL_GRADES: Grade[] = [
  Grade.JUNIOR,
  Grade.MIDDLE,
  Grade.MIDDLE_PLUS,
  Grade.SENIOR,
  Grade.LEAD,
  Grade.PRINCIPAL,
];

export interface CandidateSeed {
  email: string;
  firstName: string;
  lastName: string;
  middleName: string | null;
  phone: string;
  city: string;
  bio: string;
  specialization: Specialization;
  declaredGrade: Grade;
  verifiedGrade: Grade | null;
  stack: string[];
  roles: string[];
  softSkills: string[];
  experienceYears: number;
  showEmail: boolean;
  showPhone: boolean;
  showFullName: boolean;
  consentToProcessing: boolean;
  testStatus: TestStatus;
  lastTestDate: Date | null;
  testCooldownUntil: Date | null;
  lastTestScore: number | null;
  fspId: string | null;
  achievementNames: string[];
  isDemo?: boolean;
}

const DAY = 86_400_000;

export function buildCandidates(count: number): CandidateSeed[] {
  const rng = makeRng(20261005);
  const candidates: CandidateSeed[] = [];

  // Демо-аккаунты из VALIDATION.md — всегда присутствуют
  candidates.push({
    email: 'test@candidate.com',
    firstName: 'Иван',
    lastName: 'Петров',
    middleName: 'Сергеевич',
    phone: '+7 916 123-45-67',
    city: 'Москва',
    bio: 'Backend-разработчик с 4 годами опыта. Работал над платёжным шлюзом и системой очередей: Node.js, TypeScript, PostgreSQL, Redis, Docker. Интересуют задачи в highload и продуктовой разработке.',
    specialization: Specialization.BACKEND,
    declaredGrade: Grade.MIDDLE,
    verifiedGrade: Grade.MIDDLE,
    stack: ['Node.js', 'TypeScript', 'PostgreSQL', 'Redis', 'Docker', 'REST API'],
    roles: ['Backend-разработчик'],
    softSkills: ['коммуникация', 'аналитическое мышление'],
    experienceYears: 4,
    showEmail: false,
    showPhone: false,
    showFullName: true,
    consentToProcessing: true,
    testStatus: TestStatus.PASSED,
    lastTestDate: new Date(Date.now() - 120 * DAY),
    testCooldownUntil: null, // демо-кандидат всегда может начать тест
    lastTestScore: 78.5,
    fspId: 'FSP-1000001',
    achievementNames: ['Хакатон «Цифровая экономика»'],
    isDemo: true,
  });

  candidates.push({
    email: 'demo2@candidate.com',
    firstName: 'Дарья',
    lastName: 'Смирнова',
    middleName: 'Александровна',
    phone: '+7 921 555-12-34',
    city: 'Санкт-Петербург',
    bio: 'Frontend-разработчик с 7 годами опыта: React, TypeScript, Next.js, GraphQL. Вела интерфейс CRM и конструктор отчётов, наставляла junior-разработчиков.',
    specialization: Specialization.FRONTEND,
    declaredGrade: Grade.SENIOR,
    verifiedGrade: Grade.SENIOR,
    stack: ['React', 'TypeScript', 'Next.js', 'CSS', 'GraphQL', 'Storybook'],
    roles: ['Frontend-разработчик', 'Тимлид'],
    softSkills: ['наставничество', 'организация'],
    experienceYears: 7,
    showEmail: true,
    showPhone: false,
    showFullName: true,
    consentToProcessing: true,
    testStatus: TestStatus.UPGRADED,
    lastTestDate: new Date(Date.now() - 40 * DAY),
    testCooldownUntil: new Date(Date.now() + 50 * DAY),
    lastTestScore: 91,
    fspId: 'FSP-1000002',
    achievementNames: ['Сертификат Meta Front-End Developer'],
    isDemo: true,
  });

  candidates.push({
    email: 'demo3@candidate.com',
    firstName: 'Максим',
    lastName: 'Козлов',
    middleName: 'Владимирович',
    phone: '+7 903 777-88-99',
    city: 'Казань',
    bio: 'DevOps-инженер с 5 годами опыта: Kubernetes, Docker, Terraform, Ansible, Prometheus. Поднял Kubernetes-кластер и пайплайн CI/CD, настроил мониторинг и алертинг.',
    specialization: Specialization.DEVOPS,
    declaredGrade: Grade.MIDDLE_PLUS,
    verifiedGrade: Grade.MIDDLE_PLUS,
    stack: ['Kubernetes', 'Docker', 'Terraform', 'Ansible', 'Prometheus', 'GitLab CI'],
    roles: ['DevOps-инженер', 'SRE'],
    softSkills: ['ответственность', 'внимание к деталям'],
    experienceYears: 5,
    showEmail: false,
    showPhone: false,
    showFullName: true,
    consentToProcessing: true,
    testStatus: TestStatus.PASSED,
    lastTestDate: new Date(Date.now() - 75 * DAY),
    testCooldownUntil: null,
    lastTestScore: 84,
    fspId: 'FSP-1000003',
    achievementNames: ['Сертификат Certified Kubernetes Administrator'],
    isDemo: true,
  });

  const demoCount = candidates.length;
  let generated = 0;
  let attempt = 0;
  // Счётчики по специализациям: гарантируем полный разброс грейдов в каждой
  // (без этого случайность оставляет часть специализаций, например BACKEND, без SENIOR,
  //  и жёсткий фильтр грейда ±1 не находит по ним кандидатов).
  const perSpecIndex = new Map<string, number>();
  const seenGrades = new Map<string, Set<Grade>>();
  while (generated < count - demoCount && attempt < count * 5) {
    attempt++;
    const spec = SPECIALIZATIONS[attempt % SPECIALIZATIONS.length];
    const male = rng() < 0.52;
    const firstName = male ? pick(rng, FIRST_NAMES_M) : pick(rng, FIRST_NAMES_F);
    const lastName = male ? pick(rng, LAST_NAMES_M) : pick(rng, LAST_NAMES_F);
    const middleName = male ? pick(rng, MIDDLE_M) : pick(rng, MIDDLE_F);
    const specIndex = perSpecIndex.get(spec) ?? 0;
    perSpecIndex.set(spec, specIndex + 1);
    const seen = seenGrades.get(spec) ?? new Set<Grade>();
    seenGrades.set(spec, seen);
    const declared =
      seen.size < ALL_GRADES.length ? ALL_GRADES[specIndex % ALL_GRADES.length] : pick(rng, GRADE_POOL);
    seen.add(declared);
    const tested = rng() < 0.7;
    const gradeShift = tested && rng() < 0.18 ? (rng() < 0.5 ? -1 : 1) : 0;
    const order = [Grade.JUNIOR, Grade.MIDDLE, Grade.MIDDLE_PLUS, Grade.SENIOR, Grade.LEAD, Grade.PRINCIPAL];
    const declaredIndex = order.indexOf(declared);
    const verifiedIndex = tested ? Math.max(0, Math.min(order.length - 1, declaredIndex + gradeShift)) : -1;
    const stack = sample(rng, STACKS[spec], int(rng, 4, Math.min(6, STACKS[spec].length)));
    const experienceYears = declared === Grade.JUNIOR ? int(rng, 0, 2) : int(rng, 3, 12);
    const specLabel = SPEC_LABELS[spec];
    const project = pick(rng, SPEC_PROJECTS[spec]);

    const daysSinceTest = int(rng, 1, 400);
    const score = tested ? int(rng, 58, 98) : null;
    const testedRecently = tested && daysSinceTest <= 90;

    candidates.push({
      email: `candidate${String(generated + 1).padStart(3, '0')}@example.com`,
      firstName,
      lastName,
      middleName,
      phone: `+7 9${int(rng, 0, 9)}${int(rng, 10, 99)} ${int(rng, 100, 999)}-${int(rng, 10, 99)}-${int(rng, 10, 99)}`,
      city: pick(rng, CITIES),
      bio: `${specLabel}: ${experienceYears} ${experienceYears === 1 ? 'год' : experienceYears < 5 ? 'года' : 'лет'} опыта. Работал над ${project}, использовал ${stack.slice(0, 3).join(', ')}. ${pick(rng, ['Умею разбираться в чужом коде и документировать решения.', 'Люблю измеримые результаты и прозрачную коммуникацию.', 'Опыт работы в продуктовой команде и работе с релизами.', 'Практикую код-ревью и автоматизацию рутинных процессов.'])}`,
      specialization: spec,
      declaredGrade: declared,
      verifiedGrade: tested ? order[verifiedIndex] : null,
      stack,
      roles: pick(rng, ROLES[spec]),
      softSkills: pick(rng, SOFT_SKILLS),
      experienceYears,
      showEmail: rng() < 0.3,
      showPhone: rng() < 0.2,
      showFullName: rng() < 0.9,
      consentToProcessing: true,
      testStatus: tested
        ? gradeShift > 0
          ? TestStatus.UPGRADED
          : gradeShift < 0
            ? TestStatus.FAILED
            : TestStatus.PASSED
        : TestStatus.NOT_STARTED,
      lastTestDate: tested ? new Date(Date.now() - daysSinceTest * DAY) : null,
      testCooldownUntil: testedRecently ? new Date(Date.now() + (90 - daysSinceTest) * DAY) : null,
      lastTestScore: score,
      fspId: rng() < 0.25 ? `FSP-${int(rng, 200000, 999999)}` : null,
      achievementNames: rng() < 0.35 ? sampleCatalogue(rng) : [],
    });
    generated++;
  }

  return candidates;
}

function sampleCatalogue(rng: () => number): string[] {
  const names = [
    'Хакатон «Цифровая экономика»',
    'Сертификат AWS Certified Solutions Architect',
    'Чемпионат России по программированию, сезон 2023',
    'Сертификат Certified Kubernetes Administrator',
    'Вклад в open-source: 1000+ звёзд',
    'Финалист «Инженерного кубка»',
    'Сертификат ISTQB Foundation Level',
    'Ментор образовательного курса ФСП',
    'Олимпиада «Разработчик Олимпийских чемпионов»',
    'Сертификат Yandex Practicum: Backend-разработка',
  ];
  const count = int(rng, 1, 3);
  const picked = new Set<string>();
  while (picked.size < count) picked.add(pick(rng, names));
  return [...picked];
}
