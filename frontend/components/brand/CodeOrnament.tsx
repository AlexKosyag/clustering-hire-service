'use client';

import { useId } from 'react';

/**
 * Фирменный орнамент ФСП — полоса из код-символов
 * ({ } [ ] / < > ▲ и волны) в синем, красном и белом.
 * Стиль по брендбуку ФСП (метафора «код как народная традиция»:
 * орнаменты из кодовых символов).
 */

const BLUE = '#402ffd';
const RED = '#ec1d35';
const WHITE = '#ededed';

const TILE_W = 192;
const TILE_H = 24;

/** Один «тайл» орнамента: набор код-символов на сетке 192×24. */
function OrnamentTile({ x, y }: { x: number; y: number }) {
  return (
    <g transform={`translate(${x}, ${y})`}>
      {/* [ / ] — синие скобки с красным слэшем */}
      <path d="M14 3 H8 V21 H14" fill="none" stroke={BLUE} strokeWidth={3} strokeLinecap="square" />
      <line x1="19" y1="3" x2="13" y2="21" stroke={RED} strokeWidth={3} strokeLinecap="square" />
      <circle cx={11} cy={12} r={1.8} fill={WHITE} />

      {/* { } — синие фигурные скобки */}
      <path
        d="M40 4 c-3 0 -3 3 -3 4 c0 2 -2 4 -4 4 c2 0 4 2 4 4 c0 1 0 4 3 4"
        fill="none"
        stroke={BLUE}
        strokeWidth={3}
        strokeLinecap="round"
      />
      <path
        d="M56 4 c3 0 3 3 3 4 c0 2 2 4 4 4 c-2 0 -4 2 -4 4 c0 1 0 4 -3 4"
        fill="none"
        stroke={BLUE}
        strokeWidth={3}
        strokeLinecap="round"
      />
      <circle cx={48} cy={12} r={2.2} fill={WHITE} />

      {/* ▲ — белый треугольник */}
      <path d="M72 20 L78 6 L84 20 Z" fill={WHITE} />

      {/* волна — синяя */}
      <path
        d="M92 14 q3 -6 6 0 q3 6 6 0 q3 -6 6 0"
        fill="none"
        stroke={BLUE}
        strokeWidth={3}
        strokeLinecap="round"
      />

      {/* красный Г-образный уголок */}
      <path d="M118 4 h8 v6 h-6 v10" fill="none" stroke={RED} strokeWidth={3} strokeLinecap="square" />

      {/* красный зигзаг */}
      <path
        d="M134 8 l4 5 l4 -5 l4 5"
        fill="none"
        stroke={RED}
        strokeWidth={3}
        strokeLinecap="square"
        strokeLinejoin="miter"
      />
      <circle cx={138} cy={18} r={1.8} fill={WHITE} />

      {/* < > — синие уголки */}
      <path d="M158 6 l-6 6 l6 6" fill="none" stroke={BLUE} strokeWidth={3} strokeLinecap="square" />
      <path d="M166 6 l6 6 l-6 6" fill="none" stroke={BLUE} strokeWidth={3} strokeLinecap="square" />

      {/* белые точки */}
      <circle cx={180} cy={8} r={2} fill={WHITE} />
      <circle cx={186} cy={16} r={1.4} fill={WHITE} />
    </g>
  );
}

interface Props {
  /** Высота полосы в px. */
  height?: number;
  /** Непрозрачность (0–1). */
  opacity?: number;
  className?: string;
  style?: React.CSSProperties;
}

/**
 * Горизонтальная полоса-орнамент, повторяющаяся по ширине.
 * Размещать под шапкой, в футере и на публичных страницах.
 */
export default function CodeOrnament({ height = 28, opacity = 1, className, style }: Props) {
  const patternId = `fsp-orn-band-${useId().replace(/:/g, '')}`;
  // Тайл масштабируется ровно под высоту полосы — ничего не обрезается,
  // а паттерн бесшовно повторяется по любой ширине контейнера
  const scale = height / TILE_H;
  return (
    <div
      className={className}
      aria-hidden
      style={{
        width: '100%',
        height,
        overflow: 'hidden',
        opacity,
        lineHeight: 0,
        ...style,
      }}
    >
      <svg
        width="100%"
        height={height}
        xmlns="http://www.w3.org/2000/svg"
        style={{ display: 'block' }}
      >
        <defs>
          <pattern
            id={patternId}
            width={TILE_W}
            height={TILE_H}
            patternUnits="userSpaceOnUse"
            patternTransform={`scale(${scale})`}
          >
            <OrnamentTile x={0} y={0} />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill={`url(#${patternId})`} />
      </svg>
    </div>
  );
}

/**
 * Сплошной фоновый узор из тех же код-символов — для hero-блоков.
 * Низкая прозрачность, не мешает контенту.
 */
export function CodeOrnamentBackdrop({
  opacity = 0.14,
  className,
  style,
}: {
  opacity?: number;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <div
      className={className}
      aria-hidden
      style={{
        position: 'absolute',
        inset: 0,
        overflow: 'hidden',
        opacity,
        pointerEvents: 'none',
        ...style,
      }}
    >
      <svg width="100%" height="100%" xmlns="http://www.w3.org/2000/svg" style={{ display: 'block' }}>
        <defs>
          <pattern id="fsp-orn-pattern" width={TILE_W} height={TILE_H * 2} patternUnits="userSpaceOnUse">
            <OrnamentTile x={0} y={0} />
            <g transform={`translate(${TILE_W / 2}, ${TILE_H})`}>
              <OrnamentTile x={0} y={0} />
            </g>
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#fsp-orn-pattern)" />
      </svg>
    </div>
  );
}
