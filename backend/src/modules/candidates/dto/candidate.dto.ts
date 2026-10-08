import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { Grade, Specialization } from '@prisma/client';
import { Type } from 'class-transformer';
import {
  IsArray,
  IsBoolean,
  IsEnum,
  IsNumber,
  IsOptional,
  IsString,
  MaxLength,
  Min,
} from 'class-validator';

export class UpdateCandidateProfileDto {
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(60) firstName?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(60) lastName?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(60) middleName?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(40) phone?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(60) city?: string;
  @ApiPropertyOptional() @IsOptional() @IsString() @MaxLength(4000) bio?: string;

  @ApiPropertyOptional({ enum: Specialization }) @IsOptional() @IsEnum(Specialization) specialization?: Specialization;
  @ApiPropertyOptional({ enum: Grade }) @IsOptional() @IsEnum(Grade) declaredGrade?: Grade;

  @ApiPropertyOptional({ type: [String] }) @IsOptional() @IsArray() @IsString({ each: true }) @MaxLength(60, { each: true }) stack?: string[];
  @ApiPropertyOptional({ type: [String] }) @IsOptional() @IsArray() @IsString({ each: true }) @MaxLength(60, { each: true }) roles?: string[];
  @ApiPropertyOptional({ type: [String] }) @IsOptional() @IsArray() @IsString({ each: true }) @MaxLength(60, { each: true }) softSkills?: string[];

  @ApiPropertyOptional({ example: 4 }) @IsOptional() @Type(() => Number) @IsNumber() @Min(0) experienceYears?: number;

  @ApiPropertyOptional() @IsOptional() @IsBoolean() showEmail?: boolean;
  @ApiPropertyOptional() @IsOptional() @IsBoolean() showPhone?: boolean;
  @ApiPropertyOptional() @IsOptional() @IsBoolean() showFullName?: boolean;
  @ApiPropertyOptional() @IsOptional() @IsBoolean() consentToProcessing?: boolean;
}
