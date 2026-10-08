'use client';

import { InboxOutlined, LoadingOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { App, Button, Card, Progress, Space, Typography, Upload } from 'antd';
import { useState } from 'react';
import { api, errMsg } from '@/lib/api';
import { theme } from '@/lib/theme';

const { Text, Paragraph } = Typography;
const { Dragger } = Upload;

interface ResumeUploadProps {
  currentUrl?: string | null;
  parsed?: any;
  onUploaded?: (data: any) => void;
}

/**
 * PDF resume upload with parsing preview.
 */
export default function ResumeUpload({ currentUrl, parsed, onUploaded }: ResumeUploadProps) {
  const { message } = App.useApp();
  const [uploading, setUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<any>(null);

  const handleUpload = async (file: File) => {
    if (file.type !== 'application/pdf') {
      message.error('Резюме должно быть в формате PDF');
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      message.error('Максимальный размер — 5 МБ');
      return;
    }

    setUploading(true);
    const formData = new FormData();
    formData.append('file', file);

    try {
      const { data } = await api.post('/candidates/profile/resume', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      setUploadResult(data);
      message.success('Резюме загружено и распарсено!');
      onUploaded?.(data);
    } catch (error) {
      message.error(errMsg(error));
    } finally {
      setUploading(false);
    }
  };

  return (
    <div>
      {/* Current resume */}
      {currentUrl && !uploadResult && (
        <div
          style={{
            padding: 16,
            background: theme.colors.successLight,
            borderRadius: theme.radius.md,
            marginBottom: 16,
            display: 'flex',
            alignItems: 'center',
            gap: 12,
          }}
        >
          <CheckCircleOutlined style={{ color: theme.colors.success, fontSize: 20 }} />
          <div>
            <Text strong>Резюме загружено</Text>
            <div style={{ fontSize: 12, color: theme.colors.textSecondary }}>
              <a href={currentUrl} target="_blank" rel="noopener noreferrer">
                Открыть PDF
              </a>
            </div>
          </div>
        </div>
      )}

      {/* Upload area */}
      <Dragger
        accept=".pdf"
        multiple={false}
        showUploadList={false}
        customRequest={async ({ file, onSuccess, onError }) => {
          try {
            await handleUpload(file as File);
            onSuccess?.(null);
          } catch (e) {
            onError?.(e as Error);
          }
        }}
        disabled={uploading}
        style={{
          border: `2px dashed ${theme.colors.border}`,
          borderRadius: theme.radius.lg,
          padding: 24,
        }}
      >
        <p className="ant-upload-drag-icon">
          {uploading ? <LoadingOutlined style={{ fontSize: 48, color: theme.colors.primary }} /> : <InboxOutlined style={{ fontSize: 48 }} />}
        </p>
        <p className="ant-upload-text">
          {uploading ? 'Загрузка и парсинг...' : 'Нажмите или перетащите PDF резюме'}
        </p>
        <p className="ant-upload-hint" style={{ color: theme.colors.textMuted }}>
          Мы извлечём навыки, опыт и контакты для заполнения профиля
        </p>
      </Dragger>

      {/* Parsed result */}
      {uploadResult?.parsed && (
        <Card
          style={{
            marginTop: 16,
            borderRadius: theme.radius.md,
            border: `1px solid ${theme.colors.success}`,
          }}
          title={
            <Space>
              <CheckCircleOutlined style={{ color: theme.colors.success }} />
              <span>Извлечено из резюме</span>
            </Space>
          }
        >
          {uploadResult.parsed.name && (
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Имя:
              </Text>{' '}
              <Text strong>{uploadResult.parsed.name}</Text>
            </div>
          )}
          {uploadResult.parsed.email && (
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Email:
              </Text>{' '}
              <Text>{uploadResult.parsed.email}</Text>
            </div>
          )}
          {uploadResult.parsed.city && (
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Город:
              </Text>{' '}
              <Text>{uploadResult.parsed.city}</Text>
            </div>
          )}
          {uploadResult.parsed.skills?.length > 0 && (
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                Навыки ({uploadResult.parsed.skills.length}):
              </Text>
              <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                {uploadResult.parsed.skills.slice(0, 15).map((skill: string) => (
                  <span
                    key={skill}
                    style={{
                      padding: '2px 8px',
                      background: theme.colors.primaryBg,
                      color: theme.colors.primary,
                      borderRadius: 4,
                      fontSize: 12,
                    }}
                  >
                    {skill}
                  </span>
                ))}
              </div>
            </div>
          )}
          {uploadResult.autoFilled && (
            <div
              style={{
                marginTop: 16,
                padding: 12,
                background: theme.colors.infoLight,
                borderRadius: theme.radius.sm,
                fontSize: 12,
              }}
            >
              <Text type="secondary">
                Автоматически заполнено: {Object.entries(uploadResult.autoFilled)
                  .filter(([, v]) => v)
                  .map(([k]) => k)
                  .join(', ')}
              </Text>
            </div>
          )}
        </Card>
      )}
    </div>
  );
}