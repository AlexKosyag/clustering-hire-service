import { Injectable, Logger } from '@nestjs/common';

export interface ParsedResume {
  name?: string;
  email?: string;
  phone?: string;
  city?: string;
  skills: string[];
  experience: ExperienceEntry[];
  education: EducationEntry[];
  summary?: string;
  rawText: string;
}

export interface ExperienceEntry {
  title?: string;
  company?: string;
  period?: string;
  description?: string;
}

export interface EducationEntry {
  institution?: string;
  degree?: string;
  year?: string;
}

// Known tech skills for extraction
const TECH_SKILLS = [
  // Languages
  'JavaScript', 'TypeScript', 'Python', 'Java', 'Go', 'Golang', 'C\\+\\+', 'C#', 'Ruby', 'PHP',
  'Swift', 'Kotlin', 'Rust', 'Scala', 'Elixir', 'Clojure', 'Haskell', 'Lua', 'R',
  // Frontend
  'React', 'Vue', 'Angular', 'Next\\.js', 'Nuxt', 'Svelte', 'HTML', 'CSS', 'SASS', 'LESS',
  'Redux', 'MobX', 'Zustand', 'jQuery', 'Bootstrap', 'Tailwind',
  // Backend
  'Node\\.js', 'Express', 'NestJS', 'Django', 'Flask', 'FastAPI', 'Spring', 'Laravel',
  'Ruby on Rails', 'ASP\\.NET', 'Gin', 'Echo', 'Fiber',
  // Databases
  'PostgreSQL', 'MySQL', 'MongoDB', 'Redis', 'Elasticsearch', 'Cassandra', 'DynamoDB',
  'SQLite', 'MariaDB', 'ClickHouse', 'TimescaleDB', 'InfluxDB',
  // DevOps
  'Docker', 'Kubernetes', 'K8s', 'Terraform', 'Ansible', 'Jenkins', 'GitLab CI',
  'GitHub Actions', 'AWS', 'GCP', 'Azure', 'Yandex Cloud', 'Heroku',
  // Messaging
  'Kafka', 'RabbitMQ', 'ActiveMQ', 'NATS', 'Pulsar',
  // Other
  'GraphQL', 'gRPC', 'REST', 'SOAP', 'WebSocket', 'gRPC',
  'Git', 'Linux', 'Bash', 'CI/CD', 'Microservices',
  // ML/Data
  'TensorFlow', 'PyTorch', 'scikit-learn', 'Pandas', 'NumPy', 'Jupyter',
  'Machine Learning', 'Deep Learning', 'NLP', 'Computer Vision',
  // Mobile
  'React Native', 'Flutter', 'iOS', 'Android', 'Xamarin',
];

const SKILL_PATTERN = new RegExp(`\\b(${TECH_SKILLS.join('|')})\\b`, 'gi');

// Email pattern
const EMAIL_PATTERN = /[\w.-]+@[\w.-]+\.\w{2,}/;

// Phone patterns (Russian and international)
const PHONE_PATTERNS = [
  /(\+7|8)[\s-]?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}/,  // +7 XXX XXX XX XX
  /\d{3}[\s-]\d{3}[\s-]\d{2}[\s-]\d{2}/,  // XXX XXX XX XX
];

// City keywords
const CITY_KEYWORDS = [
  'Москва', 'Санкт-Петербург', 'Питер', 'СПб', 'Новосибирск', 'Екатеринбург',
  'Казань', 'Нижний Новгород', 'Челябинск', 'Самара', 'Омск', 'Ростов-на-Дону',
  'Уфа', 'Красноярск', 'Пермь', 'Воронеж', 'Краснодар', 'Сочи', 'Владивосток',
  'Minsk', 'Almaty', 'Tashkent', 'Bishkek', 'Dushanbe', 'Astana',
];

@Injectable()
export class PdfParsingService {
  private readonly logger = new Logger('PdfParsing');

  /**
   * Parse PDF buffer and extract resume data.
   */
  async parseResume(buffer: Buffer): Promise<ParsedResume> {
    let rawText = '';

    try {
      // Dynamic import to avoid SSR issues
      const pdfParseModule = await import('pdf-parse');
      const pdfParse = (pdfParseModule as any).default || pdfParseModule;
      const data = await pdfParse(buffer);
      rawText = data.text || '';
    } catch (e) {
      this.logger.error(`PDF parsing failed: ${(e as Error).message}`);
      throw new Error('Не удалось прочитать PDF файл');
    }

    // Extract fields from text
    const email = this.extractEmail(rawText);
    const phone = this.extractPhone(rawText);
    const city = this.extractCity(rawText);
    const skills = this.extractSkills(rawText);
    const experience = this.extractExperience(rawText);
    const education = this.extractEducation(rawText);
    const name = this.extractName(rawText);
    const summary = this.extractSummary(rawText);

    return {
      name,
      email,
      phone,
      city,
      skills,
      experience,
      education,
      summary,
      rawText: rawText.slice(0, 5000), // Limit raw text size
    };
  }

  private extractEmail(text: string): string | undefined {
    const match = text.match(EMAIL_PATTERN);
    return match?.[0];
  }

  private extractPhone(text: string): string | undefined {
    for (const pattern of PHONE_PATTERNS) {
      const match = text.match(pattern);
      if (match) return match[0];
    }
    return undefined;
  }

  private extractCity(text: string): string | undefined {
    for (const city of CITY_KEYWORDS) {
      if (text.toLowerCase().includes(city.toLowerCase())) {
        return city;
      }
    }
    return undefined;
  }

  private extractSkills(text: string): string[] {
    const matches = text.match(SKILL_PATTERN) || [];
    // Normalize and deduplicate
    const skills = new Set(
      matches.map((m) => {
        // Normalize casing
        const normalized = m.trim();
        // Common normalizations
        if (/node\.?js/i.test(normalized)) return 'Node.js';
        if (/react\.?js/i.test(normalized)) return 'React';
        if (/vue\.?js/i.test(normalized)) return 'Vue.js';
        if (/next\.?js/i.test(normalized)) return 'Next.js';
        if (/postgres/i.test(normalized)) return 'PostgreSQL';
        if (/mongo/i.test(normalized)) return 'MongoDB';
        if (/k8s/i.test(normalized)) return 'Kubernetes';
        if (/golang/i.test(normalized)) return 'Go';
        if (/c\+\+/i.test(normalized)) return 'C++';
        if (/c#/i.test(normalized)) return 'C#';
        if (/machine learning/i.test(normalized)) return 'Machine Learning';
        return normalized;
      }),
    );
    return Array.from(skills).slice(0, 30); // Limit to 30 skills
  }

  private extractExperience(text: string): ExperienceEntry[] {
    const entries: ExperienceEntry[] = [];

    // Look for experience section
    const experienceSection = text.match(
      /(?:опыт|experience|работа|занятость)[\s\S]*?(?=(?:образование|education|навыки|skills|$))/i,
    );

    if (experienceSection) {
      const section = experienceSection[0];
      // Split by common delimiters
      const lines = section.split(/\n\n|\n(?=[A-ZА-Я])/);

      for (const line of lines.slice(0, 5)) {
        const trimmed = line.trim();
        if (trimmed.length > 20) {
          entries.push({
            description: trimmed.slice(0, 300),
          });
        }
      }
    }

    return entries;
  }

  private extractEducation(text: string): EducationEntry[] {
    const entries: EducationEntry[] = [];

    // Look for education section
    const educationSection = text.match(
      /(?:образование|education|университет|институт|university)[\s\S]*?(?=(?:опыт|experience|навыки|skills|$))/i,
    );

    if (educationSection) {
      const section = educationSection[0];
      const lines = section.split(/\n\n|\n(?=[A-ZА-Я])/);

      for (const line of lines.slice(0, 3)) {
        const trimmed = line.trim();
        if (trimmed.length > 10) {
          entries.push({
            institution: trimmed.slice(0, 200),
          });
        }
      }
    }

    return entries;
  }

  private extractName(text: string): string | undefined {
    // Look for name at the beginning (common in resumes)
    const lines = text.split('\n').filter((l) => l.trim());

    for (const line of lines.slice(0, 5)) {
      const trimmed = line.trim();
      // Check if it looks like a name (2-3 capitalized words, no digits)
      if (/^[А-ЯЁ][а-яё]+\s+[А-ЯЁ][а-яё]+(?:\s+[А-ЯЁ][а-яё]+)?$/.test(trimmed) ||
          /^[A-Z][a-z]+\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+)?$/.test(trimmed)) {
        return trimmed;
      }
    }

    return undefined;
  }

  private extractSummary(text: string): string | undefined {
    const summaryMatch = text.match(
      /(?:о себе|about|summary|профиль|objective)[\s:]*\n([\s\S]{20,500}?)(?=\n\n|\n[A-ZА-Я]|$)/i,
    );
    return summaryMatch?.[1]?.trim();
  }
}