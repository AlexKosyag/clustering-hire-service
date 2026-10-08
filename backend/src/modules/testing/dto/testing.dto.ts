import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Type } from 'class-transformer';
import { IsEnum, IsInt, IsOptional, IsString, MaxLength, Min } from 'class-validator';

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

  @ApiPropertyOptional({ description: 'Текстовый ответ (coding/open_text/scenario)' })
  @IsOptional()
  @IsString()
  @MaxLength(20000)
  answerText?: string;

  @ApiPropertyOptional({ description: 'Затраченное время, сек' })
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(0)
  timeSpentSeconds?: number;
}
