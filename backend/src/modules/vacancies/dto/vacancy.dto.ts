import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Type } from 'class-transformer';
import {
  IsArray,
  IsEnum,
  IsInt,
  IsOptional,
  IsString,
  Max,
  MaxLength,
  Min,
  MinLength,
} from 'class-validator';

export class CreateVacancyDto {
  @ApiProperty({ example: 'Backend-разработчик (Node.js)' })
  @IsString()
  @MinLength(3)
  @MaxLength(160)
  title: string;

  @ApiProperty({ example: 'Разработка высоконагруженного API...' })
  @IsString()
  @MinLength(20)
  @MaxLength(10000)
  description: string;

  @ApiProperty({ enum: Specialization })
  @IsEnum(Specialization)
  specialization: Specialization;

  @ApiProperty({ enum: Grade, example: Grade.MIDDLE })
  @IsEnum(Grade)
  grade: Grade;

  @ApiProperty({ type: [String], example: ['Node.js', 'PostgreSQL'] })
  @IsArray()
  @IsString({ each: true })
  @MaxLength(60, { each: true })
  stack: string[];

  @ApiProperty({ example: 250000 })
  @Type(() => Number)
  @IsInt()
  @Min(0)
  salaryFrom: number;

  @ApiProperty({ example: 350000 })
  @Type(() => Number)
  @IsInt()
  @Min(0)
  salaryTo: number;

  @ApiPropertyOptional({ default: true })
  @IsOptional()
  isActive?: boolean;
}

/** Обновление вакансии: все поля опциональны (отдельный DTO — без наследования от Create). */
export class UpdateVacancyDto {
  @ApiPropertyOptional() @IsOptional() @IsString() @MinLength(3) @MaxLength(160) title?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MinLength(20) @MaxLength(10000) description?: string;
  @ApiPropertyOptional({ enum: Specialization }) @IsOptional() @IsEnum(Specialization) specialization?: Specialization;
  @ApiPropertyOptional({ enum: Grade }) @IsOptional() @IsEnum(Grade) grade?: Grade;
  @ApiPropertyOptional({ type: [String] }) @IsOptional() @IsArray() @IsString({ each: true }) stack?: string[];
  @ApiPropertyOptional() @IsOptional() @Type(() => Number) @IsInt() @Min(0) salaryFrom?: number;
  @ApiPropertyOptional() @IsOptional() @Type(() => Number) @IsInt() @Min(0) salaryTo?: number;
  @ApiPropertyOptional() @IsOptional() isActive?: boolean;
}
