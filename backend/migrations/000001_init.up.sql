-- pgvector extension (требуется для колонок vector(384))
CREATE EXTENSION IF NOT EXISTS vector;

-- CreateEnum
CREATE TYPE "UserRole" AS ENUM ('CANDIDATE', 'EMPLOYER', 'ADMIN');

-- CreateEnum
CREATE TYPE "Specialization" AS ENUM ('BACKEND', 'FRONTEND', 'FULLSTACK', 'MOBILE', 'DEVOPS', 'DATA_SCIENCE', 'QA', 'SECURITY', 'EMBEDDED', 'GAME_DEV', 'ML_AI', 'DESIGN_UX', 'MANAGEMENT');

-- CreateEnum
CREATE TYPE "Grade" AS ENUM ('JUNIOR', 'MIDDLE', 'MIDDLE_PLUS', 'SENIOR', 'LEAD', 'PRINCIPAL');

-- CreateEnum
CREATE TYPE "TestStatus" AS ENUM ('NOT_STARTED', 'IN_PROGRESS', 'COMPLETED', 'FAILED', 'PASSED', 'UPGRADED');

-- CreateEnum
CREATE TYPE "InvitationStatus" AS ENUM ('SENT', 'VIEWED', 'ACCEPTED', 'REJECTED', 'WITHDRAWN');

-- CreateEnum
CREATE TYPE "ApplicationStatus" AS ENUM ('PENDING', 'VIEWED', 'SHORTLISTED', 'REJECTED', 'WITHDRAWN');

-- CreateEnum
CREATE TYPE "SessionStatus" AS ENUM ('IN_PROGRESS', 'COMPLETED', 'TIMED_OUT', 'ABANDONED');

-- CreateEnum
CREATE TYPE "DailyTaskStatus" AS ENUM ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'SKIPPED', 'EXPIRED');

-- CreateTable
CREATE TABLE "users" (
    "id" TEXT NOT NULL,
    "email" TEXT NOT NULL,
    "password_hash" TEXT NOT NULL,
    "role" "UserRole" NOT NULL,
    "is_email_verified" BOOLEAN NOT NULL DEFAULT false,
    "email_verify_token" TEXT,
    "refresh_token" TEXT,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "users_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "candidate_profiles" (
    "id" TEXT NOT NULL,
    "user_id" TEXT NOT NULL,
    "first_name" TEXT,
    "last_name" TEXT,
    "middle_name" TEXT,
    "phone" TEXT,
    "city" TEXT,
    "bio" TEXT,
    "avatar_url" TEXT,
    "specialization" "Specialization",
    "declared_grade" "Grade",
    "verified_grade" "Grade",
    "stack" TEXT[],
    "roles" TEXT[],
    "experience_years" DOUBLE PRECISION,
    "soft_skills" TEXT[],
    "fsp_id" TEXT,
    "fsp_connected_at" TIMESTAMP(3),
    "test_status" "TestStatus" NOT NULL DEFAULT 'NOT_STARTED',
    "last_test_date" TIMESTAMP(3),
    "test_cooldown_until" TIMESTAMP(3),
    "last_test_score" DOUBLE PRECISION,
    "resume_pdf_url" TEXT,
    "resume_parsed" JSONB,
    "show_email" BOOLEAN NOT NULL DEFAULT false,
    "show_phone" BOOLEAN NOT NULL DEFAULT false,
    "show_full_name" BOOLEAN NOT NULL DEFAULT true,
    "consent_to_processing" BOOLEAN NOT NULL DEFAULT false,
    "last_match_score" DOUBLE PRECISION,
    "profile_embedding" vector(384),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "candidate_profiles_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "employer_profiles" (
    "id" TEXT NOT NULL,
    "user_id" TEXT NOT NULL,
    "company_name" TEXT NOT NULL,
    "company_description" TEXT,
    "industry" TEXT,
    "website" TEXT,
    "logo_url" TEXT,
    "contact_person" TEXT,
    "contact_email" TEXT,
    "contact_phone" TEXT,
    "is_verified" BOOLEAN NOT NULL DEFAULT false,
    "verified_at" TIMESTAMP(3),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "employer_profiles_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "vacancies" (
    "id" TEXT NOT NULL,
    "employer_id" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "specialization" "Specialization" NOT NULL,
    "grade" "Grade" NOT NULL,
    "stack" TEXT[],
    "salary_from" INTEGER NOT NULL,
    "salary_to" INTEGER NOT NULL,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "vacancies_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "employer_needs" (
    "id" TEXT NOT NULL,
    "employer_id" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "specialization" "Specialization",
    "grade" "Grade",
    "stack" TEXT[],
    "need_embedding" vector(384),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "employer_needs_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "invitations" (
    "id" TEXT NOT NULL,
    "employer_id" TEXT NOT NULL,
    "candidate_id" TEXT NOT NULL,
    "vacancy_id" TEXT,
    "message" TEXT,
    "salary_from" INTEGER NOT NULL,
    "salary_to" INTEGER NOT NULL,
    "status" "InvitationStatus" NOT NULL DEFAULT 'SENT',
    "status_updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "invitations_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "applications" (
    "id" TEXT NOT NULL,
    "candidate_id" TEXT NOT NULL,
    "vacancy_id" TEXT NOT NULL,
    "cover_letter" TEXT,
    "status" "ApplicationStatus" NOT NULL DEFAULT 'PENDING',
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "applications_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "test_categories" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "description" TEXT,
    "specialization" "Specialization" NOT NULL,

    CONSTRAINT "test_categories_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "test_questions" (
    "id" TEXT NOT NULL,
    "category_id" TEXT NOT NULL,
    "question_type" TEXT NOT NULL,
    "text" TEXT NOT NULL,
    "code_snippet" TEXT,
    "options" JSONB,
    "correct_answer" TEXT,
    "explanation" TEXT,
    "difficulty" DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    "discrimination" DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    "guessing" DOUBLE PRECISION NOT NULL DEFAULT 0.25,
    "is_unique" BOOLEAN NOT NULL DEFAULT false,
    "pool_tag" TEXT,

    CONSTRAINT "test_questions_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "test_sessions" (
    "id" TEXT NOT NULL,
    "candidate_id" TEXT NOT NULL,
    "category_id" TEXT NOT NULL,
    "status" "SessionStatus" NOT NULL DEFAULT 'IN_PROGRESS',
    "started_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMP(3),
    "time_limit_minutes" INTEGER NOT NULL DEFAULT 60,
    "score" DOUBLE PRECISION,
    "raw_score" DOUBLE PRECISION,
    "max_score" DOUBLE PRECISION,
    "grade_before" "Grade",
    "grade_after" "Grade",
    "recommendation" TEXT,
    "cheat_flags" TEXT[],
    "is_suspicious" BOOLEAN NOT NULL DEFAULT false,
    "current_difficulty" DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    "theta_estimate" DOUBLE PRECISION,

    CONSTRAINT "test_sessions_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "test_answers" (
    "id" TEXT NOT NULL,
    "session_id" TEXT NOT NULL,
    "question_id" TEXT NOT NULL,
    "answer_text" TEXT,
    "isCorrect" BOOLEAN,
    "score" DOUBLE PRECISION,
    "time_spent_seconds" INTEGER,
    "answered_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "test_answers_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "fsp_achievements" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "description" TEXT,
    "category" TEXT NOT NULL,
    "year" INTEGER,
    "place" TEXT,
    "points" INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT "fsp_achievements_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "candidate_achievements" (
    "id" TEXT NOT NULL,
    "candidate_id" TEXT NOT NULL,
    "achievement_id" TEXT NOT NULL,
    "verified_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "candidate_achievements_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "daily_tasks" (
    "id" TEXT NOT NULL,
    "candidate_id" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "difficulty" TEXT NOT NULL,
    "expected_time_minutes" INTEGER NOT NULL,
    "status" "DailyTaskStatus" NOT NULL DEFAULT 'PENDING',
    "candidate_answer" TEXT,
    "score" DOUBLE PRECISION,
    "assigned_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "due_at" TIMESTAMP(3) NOT NULL,
    "completed_at" TIMESTAMP(3),

    CONSTRAINT "daily_tasks_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "users_email_key" ON "users"("email");

-- CreateIndex
CREATE UNIQUE INDEX "candidate_profiles_user_id_key" ON "candidate_profiles"("user_id");

-- CreateIndex
CREATE UNIQUE INDEX "candidate_profiles_fsp_id_key" ON "candidate_profiles"("fsp_id");

-- CreateIndex
CREATE INDEX "candidate_profiles_specialization_verified_grade_idx" ON "candidate_profiles"("specialization", "verified_grade");

-- CreateIndex
CREATE INDEX "candidate_profiles_test_status_idx" ON "candidate_profiles"("test_status");

-- CreateIndex
CREATE INDEX "candidate_profiles_fsp_id_idx" ON "candidate_profiles"("fsp_id");

-- CreateIndex
CREATE UNIQUE INDEX "employer_profiles_user_id_key" ON "employer_profiles"("user_id");

-- CreateIndex
CREATE INDEX "vacancies_specialization_grade_idx" ON "vacancies"("specialization", "grade");

-- CreateIndex
CREATE INDEX "vacancies_is_active_idx" ON "vacancies"("is_active");

-- CreateIndex
CREATE INDEX "employer_needs_employer_id_created_at_idx" ON "employer_needs"("employer_id", "created_at");

-- CreateIndex
CREATE INDEX "invitations_candidate_id_status_idx" ON "invitations"("candidate_id", "status");

-- CreateIndex
CREATE INDEX "invitations_employer_id_status_idx" ON "invitations"("employer_id", "status");

-- CreateIndex
CREATE INDEX "applications_vacancy_id_status_idx" ON "applications"("vacancy_id", "status");

-- CreateIndex
CREATE UNIQUE INDEX "applications_candidate_id_vacancy_id_key" ON "applications"("candidate_id", "vacancy_id");

-- CreateIndex
CREATE UNIQUE INDEX "test_categories_name_key" ON "test_categories"("name");

-- CreateIndex
CREATE INDEX "test_questions_category_id_difficulty_idx" ON "test_questions"("category_id", "difficulty");

-- CreateIndex
CREATE INDEX "test_sessions_candidate_id_status_idx" ON "test_sessions"("candidate_id", "status");

-- CreateIndex
CREATE INDEX "test_answers_session_id_idx" ON "test_answers"("session_id");

-- CreateIndex
CREATE UNIQUE INDEX "test_answers_session_id_question_id_key" ON "test_answers"("session_id", "question_id");

-- CreateIndex
CREATE INDEX "fsp_achievements_category_year_idx" ON "fsp_achievements"("category", "year");

-- CreateIndex
CREATE UNIQUE INDEX "candidate_achievements_candidate_id_achievement_id_key" ON "candidate_achievements"("candidate_id", "achievement_id");

-- CreateIndex
CREATE INDEX "daily_tasks_candidate_id_status_idx" ON "daily_tasks"("candidate_id", "status");

-- AddForeignKey
ALTER TABLE "candidate_profiles" ADD CONSTRAINT "candidate_profiles_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "employer_profiles" ADD CONSTRAINT "employer_profiles_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "vacancies" ADD CONSTRAINT "vacancies_employer_id_fkey" FOREIGN KEY ("employer_id") REFERENCES "employer_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "employer_needs" ADD CONSTRAINT "employer_needs_employer_id_fkey" FOREIGN KEY ("employer_id") REFERENCES "employer_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "invitations" ADD CONSTRAINT "invitations_employer_id_fkey" FOREIGN KEY ("employer_id") REFERENCES "employer_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "invitations" ADD CONSTRAINT "invitations_candidate_id_fkey" FOREIGN KEY ("candidate_id") REFERENCES "candidate_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "invitations" ADD CONSTRAINT "invitations_vacancy_id_fkey" FOREIGN KEY ("vacancy_id") REFERENCES "vacancies"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "applications" ADD CONSTRAINT "applications_candidate_id_fkey" FOREIGN KEY ("candidate_id") REFERENCES "candidate_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "applications" ADD CONSTRAINT "applications_vacancy_id_fkey" FOREIGN KEY ("vacancy_id") REFERENCES "vacancies"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "test_questions" ADD CONSTRAINT "test_questions_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "test_categories"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "test_sessions" ADD CONSTRAINT "test_sessions_candidate_id_fkey" FOREIGN KEY ("candidate_id") REFERENCES "candidate_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "test_sessions" ADD CONSTRAINT "test_sessions_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "test_categories"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "test_answers" ADD CONSTRAINT "test_answers_session_id_fkey" FOREIGN KEY ("session_id") REFERENCES "test_sessions"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "test_answers" ADD CONSTRAINT "test_answers_question_id_fkey" FOREIGN KEY ("question_id") REFERENCES "test_questions"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "candidate_achievements" ADD CONSTRAINT "candidate_achievements_candidate_id_fkey" FOREIGN KEY ("candidate_id") REFERENCES "candidate_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "candidate_achievements" ADD CONSTRAINT "candidate_achievements_achievement_id_fkey" FOREIGN KEY ("achievement_id") REFERENCES "fsp_achievements"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "daily_tasks" ADD CONSTRAINT "daily_tasks_candidate_id_fkey" FOREIGN KEY ("candidate_id") REFERENCES "candidate_profiles"("id") ON DELETE CASCADE ON UPDATE CASCADE;

