'use client';

import {
  BellOutlined,
  DashboardOutlined,
  FileSearchOutlined,
  FormOutlined,
  IdcardOutlined,
  LogoutOutlined,
  MailOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { App, Avatar, Badge, Button, Dropdown, Layout, Menu, Spin } from 'antd';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { ReactNode, useEffect, useState } from 'react';
import CodeOrnament from '@/components/brand/CodeOrnament';
import FspLogo from '@/components/brand/FspLogo';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { theme } from '@/lib/theme';

const { Header, Sider, Content, Footer } = Layout;
const c = theme.colors;

interface Props {
  children: ReactNode;
  /** Ограничение раздела ролью. */
  requireRole?: 'CANDIDATE' | 'EMPLOYER';
}

export default function AppShell({ children, requireRole }: Props) {
  const { user, ready, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const { message } = App.useApp();
  const [unread, setUnread] = useState(0);
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    if (!ready) return;
    if (!user) {
      router.replace('/login');
      return;
    }
    if (requireRole && user.role !== requireRole) {
      message.warning('Раздел недоступен для вашей роли');
      router.replace(user.role === 'EMPLOYER' ? '/employer' : '/candidate');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, user, requireRole]);

  useEffect(() => {
    if (!user) return;
    api
      .get('/notifications')
      .then((r) => setUnread((r.data.items || []).filter((i: any) => !i.read).length))
      .catch(() => undefined);
  }, [user, pathname]);

  if (!ready || !user) {
    return (
      <div
        style={{
          height: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: c.bg,
        }}
      >
        <Spin size="large" tip="Загрузка…" />
      </div>
    );
  }

  const candidateMenu = [
    { key: '/candidate', icon: <DashboardOutlined />, label: <Link href="/candidate">Дашборд</Link> },
    { key: '/candidate/profile', icon: <IdcardOutlined />, label: <Link href="/candidate/profile">Профиль</Link> },
    { key: '/candidate/test', icon: <FormOutlined />, label: <Link href="/candidate/test">Тестирование</Link> },
    { key: '/candidate/invitations', icon: <MailOutlined />, label: <Link href="/candidate/invitations">Приглашения</Link> },
  ];

  const employerMenu = [
    { key: '/employer', icon: <DashboardOutlined />, label: <Link href="/employer">Дашборд</Link> },
    { key: '/employer/search', icon: <TeamOutlined />, label: <Link href="/employer/search">Поиск кандидатов</Link> },
    { key: '/employer/vacancies', icon: <FileSearchOutlined />, label: <Link href="/employer/vacancies">Мои вакансии</Link> },
  ];

  const menuItems = user.role === 'EMPLOYER' ? employerMenu : candidateMenu;

  return (
    <Layout style={{ minHeight: '100vh', background: c.bg }}>
      <Header
        style={{
          position: 'sticky',
          top: 0,
          zIndex: 20,
          background: c.bg,
          borderBottom: `1px solid ${c.borderLight}`,
          padding: '0 16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
        }}
      >
        <div className="site-header" style={{ width: '100%' }}>
          <Link href={user.role === 'EMPLOYER' ? '/employer' : '/candidate'} className="brand">
            <FspLogo height={26} />
            <span
              className="fsp-label"
              style={{ color: c.textMuted, borderLeft: `1px solid ${c.border}`, paddingLeft: 12 }}
            >
              Платформа ИТ-вакансий
            </span>
          </Link>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Badge count={unread} size="small">
              <Button
                type="text"
                style={{ color: c.textSecondary }}
                icon={<BellOutlined style={{ color: c.textSecondary, fontSize: 16 }} />}
                onClick={() => router.push('/notifications')}
              />
            </Badge>
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'logout',
                    icon: <LogoutOutlined />,
                    label: 'Выйти',
                    onClick: () => {
                      logout();
                      router.replace('/login');
                    },
                  },
                ],
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', color: c.textSecondary }}>
                <Avatar
                  size="small"
                  style={{ background: c.primary }}
                  icon={<UserOutlined />}
                />
                <span style={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {user.email}
                </span>
              </div>
            </Dropdown>
          </div>
        </div>
      </Header>
      <CodeOrnament height={24} opacity={0.9} />

      <Layout>
        <Sider
          collapsible
          collapsed={collapsed}
          onCollapse={setCollapsed}
          breakpoint="lg"
          theme="dark"
          style={{ borderRight: `1px solid ${c.borderLight}`, background: c.bg }}
        >
          <Menu
            mode="inline"
            theme="dark"
            selectedKeys={[pathname || '/']}
            items={menuItems}
            style={{ height: '100%', borderInlineEnd: 'none', background: c.bg }}
          />
        </Sider>
        <Layout style={{ background: c.bg }}>
          <Content className="site-content">{children}</Content>
          <Footer className="footer-note" style={{ borderTop: `1px solid ${c.borderLight}` }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
              <FspLogo height={20} />
              <span>
                Федерация спортивного программирования · Цифровая платформа-агрегатор ИТ-вакансий ·{' '}
                <a href="/api/docs" target="_blank" rel="noreferrer">
                  /api/docs
                </a>
              </span>
            </div>
          </Footer>
        </Layout>
      </Layout>
    </Layout>
  );
}
