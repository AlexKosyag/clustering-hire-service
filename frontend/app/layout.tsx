import type { Metadata } from 'next';
import './globals.css';
import Providers from './providers';

export const metadata: Metadata = {
  title: 'ФСП · Платформа ИТ-вакансий',
  description:
    'Цифровая платформа-агрегатор ИТ-вакансий Федерации профсоюзов России: адаптивное тестирование, matching кандидатов и работодателей, достижения и приглашения.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ru">
      <body>{<Providers>{children}</Providers>}</body>
    </html>
  );
}
