/**
 * РџСЂРµРґРІР°СЂРёС‚РµР»СЊРЅС‹Р№ СЂР°СЃС‡С‘С‚ СЌРјР±РµРґРґРёРЅРіРѕРІ (TF-IDF) РґР»СЏ РІСЃРµС… РєР°РЅРґРёРґР°С‚РѕРІ Рё РїРѕС‚СЂРµР±РЅРѕСЃС‚РµР№.
 * Р—Р°РїСѓСЃРє: npm run seed:embeddings
 *
 * РќСѓР¶РµРЅ, С‡С‚РѕР±С‹ semantic-РїРѕРёСЃРє СЂР°Р±РѕС‚Р°Р» СЃСЂР°Р·Сѓ РїРѕСЃР»Рµ seed'Р°, РЅРµ РґРѕР¶РёРґР°СЏСЃСЊ С„РѕРЅРѕРІРѕР№
 * РїРµСЂРµРёРЅРґРµРєСЃР°С†РёРё РїСЂРё СЃС‚Р°СЂС‚Рµ РїСЂРёР»РѕР¶РµРЅРёСЏ. Р•СЃР»Рё EMBEDDING_PROVIDER=transformers вЂ”
 * РїСЂРёР»РѕР¶РµРЅРёРµ РїСЂРё СЃС‚Р°СЂС‚Рµ РїСЂРѕРіРѕРЅРёС‚ РїРѕР»РЅСѓСЋ РїРµСЂРµРёРЅРґРµРєСЃР°С†РёСЋ РЅР° РјРѕРґРµР»Рё.
 */
import { PrismaClient } from '@prisma/client';
import { TfidfVectorizer } from '../src/modules/matching/embedding/tfidf';

const prisma = new PrismaClient();

interface Row {
  id: string;
  text: string;
}

function candidateText(row: any): string {
  return [
    row.firstName,
    row.lastName,
    row.specialization,
    row.declaredGrade,
    row.verifiedGrade,
    (row.stack || []).join(' '),
    (row.roles || []).join(' '),
    (row.softSkills || []).join(' '),
    row.city,
    row.bio,
    (row.achievements || []).join(' '),
  ]
    .filter(Boolean)
    .join(' ');
}

function needText(row: any): string {
  return [row.description, row.specialization, row.grade, (row.stack || []).join(' ')]
    .filter(Boolean)
    .join(' ');
}

async function main() {
  const candidates = await prisma.candidateProfile.findMany({
    include: { achievements: { include: { achievement: { select: { name: true } } } } },
  });
  const needs = await prisma.employerNeed.findMany();

  const candidateRows: Row[] = candidates.map((row: any) => ({
    id: row.id,
    text: candidateText({
      ...row,
      achievements: row.achievements.map((a: any) => a.achievement.name),
    }),
  }));
  const needRows: Row[] = needs.map((row) => ({ id: row.id, text: needText(row) }));

  const corpus = [...candidateRows, ...needRows].map((r) => r.text);
  const vectorizer = new TfidfVectorizer();
  vectorizer.fit(corpus);

  const started = Date.now();
  for (const row of candidateRows) {
    const vector = vectorizer.embed(row.text);
    await prisma.$executeRawUnsafe(
      `UPDATE candidate_profiles SET profile_embedding = $1::vector WHERE id = $2`,
      `[${vector.map((v) => v.toFixed(6)).join(',')}]`,
      row.id,
    );
  }
  for (const row of needRows) {
    const vector = vectorizer.embed(row.text);
    await prisma.$executeRawUnsafe(
      `UPDATE employer_needs SET need_embedding = $1::vector WHERE id = $2`,
      `[${vector.map((v) => v.toFixed(6)).join(',')}]`,
      row.id,
    );
  }

  console.log(
    `вњ… Р­РјР±РµРґРґРёРЅРіРё Р·Р°РїРёСЃР°РЅС‹: ${candidateRows.length} РєР°РЅРґРёРґР°С‚РѕРІ, ${needRows.length} РїРѕС‚СЂРµР±РЅРѕСЃС‚РµР№ Р·Р° ${Date.now() - started}ms (TF-IDF, 384-dim)`,
  );
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  })
  .finally(() => prisma.$disconnect());

