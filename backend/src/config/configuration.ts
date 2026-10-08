export default () => ({
  port: Number(process.env.PORT || 3000),
  databaseUrl: process.env.DATABASE_URL,
  redisUrl: process.env.REDIS_URL || 'redis://redis:6379',
  jwt: {
    secret: process.env.JWT_SECRET || 'dev-secret',
    refreshSecret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
    accessTokenTtl: '15m',
    refreshTokenTtlDays: 7,
  },
  minio: {
    endpoint: process.env.MINIO_ENDPOINT || 'http://minio:9000',
    accessKey: process.env.MINIO_ACCESS_KEY || 'minioadmin',
    secretKey: process.env.MINIO_SECRET_KEY || 'minioadmin',
    bucket: process.env.MINIO_BUCKET || 'fsp-platform',
  },
  embedding: {
    provider: process.env.EMBEDDING_PROVIDER || 'auto', // auto | tfidf | transformers
  },
  matching: {
    cacheTtlSeconds: 900,
  },
  testing: {
    questionsPerTest: 20,
    cooldownDays: 90,
  },
});
