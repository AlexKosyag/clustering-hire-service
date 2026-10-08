/**
 * Лексический fallback для semantic matching (384-dim, детерминированный).
 * Используется, когда недоступна ONNX-модель (сеть/HuggingFace/рантайм).
 * Интерфейс совпадает с transformers-пайплайном: text -> number[384], L2-normalized.
 */
export const EMBEDDING_DIM = 384;

function fnv1a(str: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < str.length; i++) {
    hash ^= str.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

export function tokenize(text: string): string[] {
  if (!text) return [];
  const normalized = text
    .toLowerCase()
    .replace(/ё/g, 'е')
    .replace(/[^a-zа-я0-9+#]+/g, ' ')
    .trim();
  const words = normalized.split(/\s+/).filter((w) => w.length > 1);
  const tokens = [...words];
  for (let i = 0; i < words.length - 1; i++) {
    tokens.push(`${words[i]}_${words[i + 1]}`); // биграммы для устойчивости к фразам
  }
  return tokens;
}

export class TfidfVectorizer {
  private idf = new Map<string, number>();
  private docs = 0;
  private fitted = false;

  get isFitted(): boolean {
    return this.fitted;
  }

  /** Обучает IDF на корпусе профилей/описаний. */
  fit(texts: string[]): void {
    const df = new Map<string, number>();
    for (const text of texts) {
      for (const token of new Set(tokenize(text))) {
        df.set(token, (df.get(token) || 0) + 1);
      }
    }
    this.docs = texts.length;
    this.idf = new Map();
    for (const [token, count] of df) {
      this.idf.set(token, Math.log((1 + this.docs) / (1 + count)) + 1);
    }
    this.fitted = this.docs > 0;
  }

  private idfOf(token: string): number {
    return this.idf.get(token) ?? Math.log(1 + this.docs) + 1;
  }

  /** Возвращает L2-нормированный вектор 384 dim. Никогда не бросает исключение. */
  embed(text: string): number[] {
    try {
      const tokens = tokenize(text);
      const tf = new Map<string, number>();
      for (const t of tokens) tf.set(t, (tf.get(t) || 0) + 1);

      const vector = new Array<number>(EMBEDDING_DIM).fill(0);
      for (const [token, count] of tf) {
        const weight = (1 + Math.log(count)) * this.idfOf(token);
        const hash = fnv1a(token);
        const index = hash % EMBEDDING_DIM;
        const sign = (hash & 0x80000000) >>> 31 ? -1 : 1;
        vector[index] += sign * weight;
      }
      return normalize(vector);
    } catch {
      return unitVector();
    }
  }
}

/** L2-нормализация с guard-ом от нулевой нормы (защита от NaN в cosine). */
export function normalize(vector: number[]): number[] {
  let sum = 0;
  for (const v of vector) {
    if (Number.isFinite(v)) sum += v * v;
  }
  if (sum < 1e-12 || !Number.isFinite(sum)) return unitVector();
  const norm = Math.sqrt(sum);
  return vector.map((v) => (Number.isFinite(v) ? v / norm : 0));
}

export function unitVector(): number[] {
  const vector = new Array<number>(EMBEDDING_DIM).fill(0);
  vector[0] = 1;
  return vector;
}

export function cosineSimilarity(a: number[], b: number[]): number {
  if (!a?.length || !b?.length || a.length !== b.length) return 0;
  let dot = 0;
  let na = 0;
  let nb = 0;
  for (let i = 0; i < a.length; i++) {
    if (!Number.isFinite(a[i]) || !Number.isFinite(b[i])) continue;
    dot += a[i] * b[i];
    na += a[i] * a[i];
    nb += b[i] * b[i];
  }
  if (na < 1e-12 || nb < 1e-12) return 0;
  const value = dot / (Math.sqrt(na) * Math.sqrt(nb));
  return Number.isFinite(value) ? value : 0;
}
