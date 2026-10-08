import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { IsInt, IsOptional, IsString, Max, MaxLength, Min } from 'class-validator';
import { Type } from 'class-transformer';

export class CreateInvitationDto {
  @ApiProperty({ description: 'ID кандидата' })
  @IsString()
  candidateId: string;

  @ApiPropertyOptional({ description: 'ID вакансии (опционально)' })
  @IsOptional()
  @IsString()
  vacancyId?: string;

  @ApiPropertyOptional({ example: 'Приглашаем в нашу команду backend-разработчиков' })
  @IsOptional()
  @IsString()
  @MaxLength(4000)
  message?: string;

  @ApiProperty({ example: 250000 })
  @Type(() => Number)
  @IsInt()
  @Min(0)
  @Max(10_000_000)
  salaryFrom: number;

  @ApiProperty({ example: 350000 })
  @Type(() => Number)
  @IsInt()
  @Min(0)
  @Max(10_000_000)
  salaryTo: number;
}

export class InvitationStatusDto {
  @ApiProperty({ enum: ['ACCEPTED', 'REJECTED'] })
  @IsString()
  status: 'ACCEPTED' | 'REJECTED';
}
