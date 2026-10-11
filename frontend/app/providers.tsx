'use client';

import { App, ConfigProvider, theme as antdTheme } from 'antd';
import ruRU from 'antd/locale/ru_RU';
import { ReactNode } from 'react';
import { AuthProvider } from '@/lib/auth';
import { theme } from '@/lib/theme';

const c = theme.colors;

export default function Providers({ children }: { children: ReactNode }) {
  return (
    <ConfigProvider
      locale={ruRU}
      theme={{
        // Тёмный алгоритм antd: все производные палитры (в т.ч. фоновые
        // подложки preset-тегов вроде blue-1/green-1) становятся тёмными
        algorithm: antdTheme.darkAlgorithm,
        token: {
          colorPrimary: c.primary,
          colorInfo: c.primary,
          colorSuccess: c.success,
          colorWarning: c.warning,
          colorError: c.error,
          // Тёмная база брендбука — antd выводит из неё всю палитру
          colorBgBase: c.bg,
          colorTextBase: c.text,
          colorBorder: c.border,
          colorSplit: c.borderLight,
          // Alert и прочие «фоновые» состояния — тёмные фирменные подложки,
          // чтобы текст не сливаться с фоном
          colorInfoBg: '#282650',
          colorInfoBorder: c.primary,
          colorSuccessBg: '#123326',
          colorSuccessBorder: c.success,
          colorWarningBg: '#332b18',
          colorWarningBorder: c.warning,
          colorErrorBg: '#3a2434',
          colorErrorBorder: c.error,
          borderRadius: 6,
          fontFamily: theme.fonts.sans,
          fontSize: 14,
        },
        components: {
          Layout: {
            headerBg: c.bg,
            bodyBg: c.bg,
            footerBg: c.bg,
            siderBg: c.bg,
            headerHeight: 56,
          },
          Menu: {
            darkItemBg: c.bg,
            darkSubMenuItemBg: c.bg,
            darkItemSelectedBg: 'rgba(64,47,253,0.22)',
            darkItemColor: c.textSecondary,
            darkItemSelectedColor: c.text,
            itemBg: c.bg,
            subMenuItemBg: c.bg,
            itemSelectedBg: 'rgba(64,47,253,0.22)',
            itemSelectedColor: c.text,
          },
          Card: {
            colorBgContainer: c.surface,
            colorBorderSecondary: c.border,
          },
          Table: {
            colorBgContainer: c.surface,
            headerBg: c.bg,
            rowHoverBg: c.surfaceHover,
          },
          Input: { colorBgContainer: c.bg },
          InputNumber: { colorBgContainer: c.bg },
          Select: { colorBgContainer: c.bg },
          DatePicker: { colorBgContainer: c.bg },
          Segmented: { itemSelectedBg: c.surfaceHover },
          Tabs: { inkBarColor: c.primary, itemSelectedColor: c.text },
          Button: { primaryShadow: 'none', defaultShadow: 'none' },
          Message: { contentBg: c.surface },
          Notification: { colorBgElevated: c.surface },
          Popover: { colorBgElevated: c.surface },
          Tooltip: { colorBgSpotlight: c.surface },
        },
      }}
    >
      <App>
        <AuthProvider>{children}</AuthProvider>
      </App>
    </ConfigProvider>
  );
}
