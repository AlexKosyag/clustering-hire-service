import Image from 'next/image';

interface Props {
  /** Высота логотипа в px. */
  height?: number;
  className?: string;
  style?: React.CSSProperties;
}

/**
 * Логотип ФСП (сокращённый, цветной) для тёмного фона.
 * Исходник: bb/Сокращенный лого ФСП цвет.svg → public/logo/fsp-logo-dark.svg
 */
export default function FspLogo({ height = 28, className, style }: Props) {
  return (
    <Image
      src="/logo/fsp-logo-dark.svg"
      alt="ФСП — Федерация спортивного программирования России"
      height={height}
      width={Math.round((height * 598.96) / 202.25)}
      className={className}
      priority
      unoptimized
      style={{ display: 'block', height: 'auto', ...style }}
    />
  );
}
