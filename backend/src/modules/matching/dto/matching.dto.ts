import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Type } from 'class-transformer';
import {
  IsArray,
  IsBoolean,
  IsEnum,
  IsInt,
  IsOptional,
  IsString,
  Max,
  MaxLength,
  Min,
  MinLength,
} from 'class-validator';

export class SearchCandidatesDto {
  @ApiPropertyOptional({ description: 'Описание потребности (семантический запрос)' })
  @IsOptional()
  @IsString()
  @MaxLength(4000)
  description?: string;

  @ApiPropertyOptional({ enum: Specialization })
  @IsOptional()
  @IsEnum(Specialization)
  specialization?: Specialization;

  @ApiPropertyOptional({ enum: Grade })
  @IsOptional()
  @IsEnum(Grade)
  grade?: Grade;

  @ApiPropertyOptional({ type: [String], example: ['Node.js', 'PostgreSQL'] })
  @IsOptional()
  @IsArray()
  @IsString({ each: true })
  @MaxLength(60, { each: true })
  stack?: string[];

  @ApiPropertyOptional({ description: 'Только кандидаты с достижениями ФСП' })
  @IsOptional()
  @IsBoolean()
  hasAchievements?: boolean;

  @ApiPropertyOptional({ default: 20 })
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(100)
  limit?: number = 20;

  @ApiPropertyOptional({ default: 0 })
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(0)
  offset?: number = 0;
}

export class EmployerNeedDto {
  @ApiProperty({ example: 'Ищем backend-разработчика на Node.js с опытом от 3 лет...' })
  @IsString()
  @MinLength(10)
  @MaxLength(4000)
  description: string;

  @ApiPropertyOptional({ enum: Specialization })
  @IsOptional()
  @IsEnum(Specialization)
  specialization?: Specialization;

  @ApiPropertyOptional({ enum: Grade })
  @IsOptional()
  @IsEnum(Grade)
  grade?: Grade;

  @ApiPropertyOptional({ type: [String] })
  @IsOptional()
  @IsArray()
  @IsString({ each: true })
  stack?: string[];
}

export class SemanticQueryDto {
  @ApiProperty({ example: 'человек который умеет строить высоконагруженные системы и знает Go' })
  @IsString()
  @MinLength(3)
  @MaxLength(2000)
  query: string;

  @ApiPropertyOptional({ default: 10 })
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(50)
  limit?: number = 10;
}
