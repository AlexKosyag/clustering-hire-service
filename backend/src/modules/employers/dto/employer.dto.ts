import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Transform, Type } from 'class-transformer';
import { IsArray, IsEnum, IsInt, IsOptional, IsString, Max, MaxLength, Min, MinLength } from 'class-validator';
import { SearchCandidatesDto } from '../../matching/dto/matching.dto';

export class UpdateEmployerProfileDto {
  @ApiPropertyOptional({ example: 'ООО ТехноСтар' }) @IsOptional() @IsString() @MaxLength(120) companyName?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(4000) companyDescription?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(80) industry?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(200) website?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(80) contactPerson?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(120) contactEmail?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(40) contactPhone?: string;
}

export class EmployerSearchQueryDto extends SearchCandidatesDto {
  @ApiPropertyOptional({ description: 'Стек через запятую: Node.js,PostgreSQL' })
  @IsOptional()
  @Transform(({ value }) => {
    if (Array.isArray(value)) return value;
    if (typeof value === 'string') return value.split(',').map((s) => s.trim()).filter(Boolean);
    return undefined;
  })
  @IsArray()
  @IsString({ each: true })
  stack?: string[];
}

/** Потребность работодателя — ключевой вход matching-движка. */
export class CreateNeedDto {
  @ApiProperty({ example: 'Ищем backend-разработчика на Node.js с опытом от 3 лет...' })
  @IsString()
  @MinLength(10)
  @MaxLength(4000)
  description: string;

  @ApiPropertyOptional({ enum: Specialization }) @IsOptional() @IsEnum(Specialization) specialization?: Specialization;
  @ApiPropertyOptional({ enum: Grade }) @IsOptional() @IsEnum(Grade) grade?: Grade;
  @ApiPropertyOptional({ type: [String] }) @IsOptional() @IsArray() @IsString({ each: true }) stack?: string[];
  @ApiPropertyOptional({ description: 'ID вакансии (опционально)' }) @IsOptional() @IsString() vacancyId?: string;
}
