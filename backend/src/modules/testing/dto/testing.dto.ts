import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Type } from 'class-transformer';
import { IsArray, IsEnum, IsInt, IsObject, IsOptional, IsString, MaxLength, Min } from 'class-validator';

export class StartTestDto {
  @ApiProperty({ enum: Specialization, example: Specialization.BACKEND })
  @IsEnum(Specialization)
  specialization: Specialization;

  @ApiPropertyOptional({ enum: Grade, description: 'Заявленный (самооценка) грейд' })
  @IsOptional()
  @IsEnum(Grade)
  declaredGrade?: Grade;
}

export class RetakeDto {
  @ApiProperty({ enum: Specialization })
  @IsEnum(Specialization)
  specialization: Specialization;

  @ApiPropertyOptional({ enum: Grade, description: 'Целевой грейд (пересдача ради повышения)' })
  @IsOptional()
  @IsEnum(Grade)
  targetGrade?: Grade;
}

export class SubmitAnswerDto {
  @ApiProperty({ description: 'ID вопроса' })
  @IsString()
  questionId: string;

  @ApiPropertyOptional({ description: 'ID варианта ответа (multiple_choice)' })
  @IsOptional()
  @IsString()
  @MaxLength(20)
  answer?: string;

  /** V2: ID выбранного варианта (multiple_choice). */
  @ApiPropertyOptional({ description: 'V2: ID выбранного варианта (multiple_choice)' })
  @IsOptional()
  @IsString()
  @MaxLength(40)
  selectedOptionId?: string;

  @ApiPropertyOptional({ description: 'Текстовый ответ (coding/open_text/scenario)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  answerText?: string;

  /** V2: код решения (code_execution). */
  @ApiPropertyOptional({ description: 'V2: код решения (code_execution)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  code?: string;

  /** V2: исправленный код (code_review). */
  @ApiPropertyOptional({ description: 'V2: исправленный код (code_review)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  fixedCode?: string;

  /** V2: найденные баги (code_review). */
  @ApiPropertyOptional({ description: 'V2: найденные баги (code_review)', type: [String] })
  @IsOptional()
  @IsArray()
  @IsString({ each: true })
  bugsFound?: string[];

  /** V2: описание решения (system_design). */
  @ApiPropertyOptional({ description: 'V2: описание решения (system_design)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  designDescription?: string;

  /** V2: схема/диаграмма (system_design). */
  @ApiPropertyOptional({ description: 'V2: схема/диаграмма (system_design)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  diagram?: string;

  @ApiPropertyOptional({ description: 'Затраченное время, сек' })
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(0)
  timeSpentSeconds?: number;

  /** V2: behavioral-данные для anti-cheat (keystrokes, copy-paste, tab-switch). */
  @ApiPropertyOptional({ description: 'V2: behavioral-данные для anti-cheat' })
  @IsOptional()
  @IsObject()
  behavioral?: Record<string, number>;
}
