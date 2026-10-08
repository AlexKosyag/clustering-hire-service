import {
  cosineSimilarity,
  EMBEDDING_DIM,
  normalize,
  TfidfVectorizer,
  tokenize,
  unitVector,
} from './tfidf';

describe('TF-IDF embedding (fallback semantic-слоя)', () => {
  describe('tokenize', () => {
    it('нормализует регистр и убирает пунктуацию', () => {
      const tokens = tokenize('Node.js и PostgreSQL!');
      expect(tokens).toContain('node');
      expect(tokens).toContain('postgresql');
      expect(tokens).not.toContain('и');
    });

    it('строит биграммы для словосочетаний', () => {
      const tokens = tokenize('высокая нагрузка');
      expect(tokens).toContain('высокая');
      expect(tokens).toContain('нагрузка');
      expect(tokens).toContain('высокая_нагрузка');
    });

    it('пустой ввод даёт пустой список', () => {
      expect(tokenize('')).toEqual([]);
      expect(tokenize('   ')).toEqual([]);
    });

    it('однобуквенные слова отбрасываются', () => {
      expect(tokenize('a bb c dd')).toEqual(['bb', 'dd', 'bb_dd']);
    });
  });

  describe('TfidfVectorizer', () => {
    const corpus = [
      'backend разработчик nodejs postgresql redis',
      'frontend разработчик react typescript css',
      'devops инженер kubernetes docker terraform',
    ];

    it('после fit помечен как обученный', () => {
      const v = new TfidfVectorizer();
      expect(v.isFitted).toBe(false);
      v.fit(corpus);
      expect(v.isFitted).toBe(true);
    });

    it('пустой корпус не обучает векторайзер', () => {
      const v = new TfidfVectorizer();
      v.fit([]);
      expect(v.isFitted).toBe(false);
    });

    it('возвращает вектор размерности 384 с единичной нормой', () => {
      const v = new TfidfVectorizer();
      v.fit(corpus);
      const vector = v.embed(corpus[0]);
      expect(vector).toHaveLength(EMBEDDING_DIM);
      const norm = Math.sqrt(vector.reduce((sum, x) => sum + x * x, 0));
      expect(norm).toBeCloseTo(1, 5);
      expect(vector.every((x) => Number.isFinite(x))).toBe(true);
    });

    it('одинаковые тексты дают косинусную близость ~1', () => {
      const v = new TfidfVectorizer();
      v.fit(corpus);
      const a = v.embed(corpus[0]);
      const b = v.embed(corpus[0]);
      expect(cosineSimilarity(a, b)).toBeCloseTo(1, 6);
    });

    it('релевантный текст ближе к запросу, чем нерелевантный', () => {
      const v = new TfidfVectorizer();
      v.fit([...corpus, 'ищем backend разработчика с опытом работы с redis и postgresql']);
      const query = v.embed('backend разработчик postgresql redis');
      const relevant = v.embed('ищем backend разработчика с опытом работы с redis и postgresql');
      const unrelated = v.embed('frontend разработчик react typescript css');
      expect(cosineSimilarity(query, relevant)).toBeGreaterThan(cosineSimilarity(query, unrelated));
    });

    it('работает и до вызова fit (деградация, а не исключение)', () => {
      const v = new TfidfVectorizer();
      const vector = v.embed('nodejs backend');
      expect(vector).toHaveLength(EMBEDDING_DIM);
      expect(vector.every((x) => Number.isFinite(x))).toBe(true);
    });
  });

  describe('normalize / unitVector', () => {
    it('нормализует в единичную длину', () => {
      const result = normalize([3, 4]);
      expect(result[0]).toBeCloseTo(0.6, 6);
      expect(result[1]).toBeCloseTo(0.8, 6);
    });

    it('guard от нулевого и нечислового вектора', () => {
      expect(normalize([0, 0, 0])).toEqual(unitVector());
      expect(normalize([Number.NaN, 0])).toEqual(unitVector());
      const guarded = normalize([Number.POSITIVE_INFINITY, 1]);
      expect(guarded.every((x) => Number.isFinite(x))).toBe(true);
      expect(guarded).toHaveLength(2);
    });

    it('заменяет NaN-компоненты нулями', () => {
      const result = normalize([1, Number.NaN, 1]);
      expect(result.every((x) => Number.isFinite(x))).toBe(true);
    });
  });

  describe('cosineSimilarity', () => {
    it('единичный вектор сам с собой даёт 1', () => {
      const v = unitVector();
      expect(cosineSimilarity(v, v)).toBeCloseTo(1, 10);
    });

    it('ортогональные векторы дают 0', () => {
      expect(cosineSimilarity([1, 0], [0, 1])).toBeCloseTo(0, 10);
    });

    it('защищает от разной длины и пустых входов', () => {
      expect(cosineSimilarity([1, 0], [1])).toBe(0);
      expect(cosineSimilarity([], [])).toBe(0);
      expect(cosineSimilarity(undefined as any, [1])).toBe(0);
    });

    it('игнорирует нечисловые компоненты', () => {
      const value = cosineSimilarity([1, Number.NaN], [1, 5]);
      expect(Number.isFinite(value)).toBe(true);
      expect(value).toBeCloseTo(1, 6);
    });
  });
});
