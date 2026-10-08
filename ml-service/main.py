"""
FSP Platform — ML Service for Semantic Matching V2

Endpoints:
  POST /embed           → generate single embedding (768-dim)
  POST /embed/batch     → batch embeddings
  POST /ner             → named entity recognition
  POST /rerank          → cross-encoder re-ranking
  POST /parse-query     → full query understanding
  GET  /health          → model status

Models:
  - sentence-transformers/paraphrase-multilingual-mpnet-base-v2 (768-dim)
  - cross-encoder/ms-marco-MiniLM-L-6-v2 (re-ranking)
  - spaCy ru_core_news_sm (NER)
"""

import asyncio
import hashlib
import logging
import os
import time
from contextlib import asynccontextmanager
from typing import Any, Optional

import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ml-service")

# ─────────────────────────── Configuration ───────────────────────────

EMBEDDING_MODEL = os.getenv(
    "MODEL_NAME",
    "sentence-transformers/paraphrase-multilingual-mpnet-base-v2",
)
RERANKER_MODEL = os.getenv(
    "RERANKER_NAME",
    "cross-encoder/ms-marco-MiniLM-L-6-v2",
)
CACHE_SIZE = int(os.getenv("CACHE_SIZE", "2000"))
REDIS_URL = os.getenv("REDIS_URL", None)

EMBEDDING_DIM = 768

# ─────────────────────────── Models (loaded at startup) ───────────────────────────

class ModelRegistry:
    embedding_model = None
    reranker_model = None
    spacy_nlp = None
    embedding_ready = False
    reranker_ready = False
    ner_ready = False

    # In-memory LRU cache for embeddings
    _embed_cache: dict[str, list[float]] = {}

    @classmethod
    def cache_key(cls, text: str) -> str:
        return hashlib.sha256(text.encode("utf-8")).hexdigest()[:32]

    @classmethod
    def get_cached(cls, key: str) -> Optional[list[float]]:
        return cls._embed_cache.get(key)

    @classmethod
    def set_cached(cls, key: str, value: list[float]) -> None:
        if len(cls._embed_cache) >= CACHE_SIZE:
            # Evict oldest (simple FIFO)
            oldest = next(iter(cls._embed_cache))
            del cls._embed_cache[oldest]
        cls._embed_cache[key] = value


registry = ModelRegistry()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load models asynchronously at startup."""
    logger.info("Starting ML Service — loading models in background...")

    async def load_embedding():
        try:
            from sentence_transformers import SentenceTransformer

            t0 = time.time()
            registry.embedding_model = SentenceTransformer(EMBEDDING_MODEL)
            registry.embedding_ready = True
            logger.info(
                f"Embedding model loaded: {EMBEDDING_MODEL} "
                f"({time.time() - t0:.1f}s, dim={EMBEDDING_DIM})"
            )
        except Exception as e:
            logger.error(f"Failed to load embedding model: {e}")

    async def load_reranker():
        try:
            from sentence_transformers import CrossEncoder

            t0 = time.time()
            registry.reranker_model = CrossEncoder(RERANKER_MODEL)
            registry.reranker_ready = True
            logger.info(f"Reranker model loaded: {RERANKER_MODEL} ({time.time() - t0:.1f}s)")
        except Exception as e:
            logger.error(f"Failed to load reranker model: {e}")

    async def load_spacy():
        try:
            import spacy

            t0 = time.time()
            try:
                registry.spacy_nlp = spacy.load("ru_core_news_sm")
            except OSError:
                logger.info("Downloading spaCy model ru_core_news_sm...")
                from spacy.cli import download

                download("ru_core_news_sm")
                registry.spacy_nlp = spacy.load("ru_core_news_sm")
            registry.ner_ready = True
            logger.info(f"spaCy NER loaded ({time.time() - t0:.1f}s)")
        except Exception as e:
            logger.error(f"Failed to load spaCy: {e}")

    # Load in parallel
    await asyncio.gather(load_embedding(), load_reranker(), load_spacy())
    logger.info(
        f"Models ready — embedding={registry.embedding_ready}, "
        f"reranker={registry.reranker_ready}, ner={registry.ner_ready}"
    )

    yield

    logger.info("Shutting down ML Service")


# ─────────────────────────── FastAPI App ───────────────────────────

app = FastAPI(
    title="FSP ML Service",
    description="Semantic Matching V2 — embeddings, re-ranking, NER",
    version="2.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


# ─────────────────────────── Schemas ───────────────────────────

class EmbedRequest(BaseModel):
    text: str = Field(..., max_length=8192)


class EmbedBatchRequest(BaseModel):
    texts: list[str] = Field(..., max_length=100)
    normalize: bool = True


class EmbedResponse(BaseModel):
    embedding: list[float]
    dim: int
    provider: str  # "transformers" | "tfidf-fallback"
    cached: bool = False
    latency_ms: float


class EmbedBatchResponse(BaseModel):
    embeddings: list[list[float]]
    dim: int
    provider: str
    latency_ms: float


class NERRequest(BaseModel):
    text: str = Field(..., max_length=8192)


class NEREntity(BaseModel):
    text: str
    label: str  # PER, ORG, LOC, TECH, etc.
    start: int
    end: int


class NERResponse(BaseModel):
    entities: list[NEREntity]
    latency_ms: float


class RerankRequest(BaseModel):
    query: str
    documents: list[str] = Field(..., max_length=200)
    top_k: Optional[int] = None


class RerankResult(BaseModel):
    index: int
    score: float
    document: str


class RerankResponse(BaseModel):
    results: list[RerankResult]
    latency_ms: float


class ParseQueryRequest(BaseModel):
    query: str = Field(..., max_length=4096)


class ParsedQuery(BaseModel):
    specialization: Optional[str] = None
    seniority: Optional[str] = None
    required_skills: list[str] = []
    preferred_skills: list[str] = []
    context: Optional[str] = None
    intent: str = "hire"


class ParseQueryResponse(BaseModel):
    parsed: ParsedQuery
    embedding: list[float]
    embedding_provider: str
    latency_ms: float


# ─────────────────────────── Skill Dictionary ───────────────────────────

SKILL_SYNONYMS: dict[str, list[str]] = {
    "javascript": ["js", "node.js", "nodejs", "node", "ecmascript", "es6", "typescript", "ts"],
    "python": ["py", "django", "flask", "fastapi"],
    "go": ["golang"],
    "java": ["kotlin", "spring", "spring boot"],
    "csharp": ["c#", ".net", "dotnet", "asp.net"],
    "cpp": ["c++", "c plus plus"],
    "react": ["reactjs", "react.js"],
    "vue": ["vuejs", "vue.js"],
    "angular": ["angularjs"],
    "postgresql": ["postgres", "pg"],
    "mysql": ["mariadb"],
    "mongodb": ["mongo", "nosql"],
    "redis": ["redis cache", "key-value store"],
    "docker": ["containers", "containerization"],
    "kubernetes": ["k8s", "kube"],
    "aws": ["amazon web services"],
    "gcp": ["google cloud"],
    "azure": ["microsoft azure"],
    "microservices": ["micro-services", "micro services"],
    "rest": ["rest api", "restful", "rest-api"],
    "graphql": ["gql"],
    "grpc": ["grpc protocol"],
    "kafka": ["apache kafka", "message queue", "message broker"],
    "rabbitmq": ["rabbit mq", "amqp"],
    "elasticsearch": ["elastic", "opensearch"],
    "devops": ["sre", "site reliability", "infrastructure"],
    "ci/cd": ["cicd", "continuous integration", "continuous delivery"],
    "git": ["github", "gitlab", "bitbucket"],
    "agile": ["scrum", "kanban"],
    "testing": ["tdd", "bdd", "unit testing", "integration testing"],
    "security": ["infosec", "cybersecurity", "appsec"],
    "ml": ["machine learning", "ai", "artificial intelligence"],
    "data science": ["data science", "data scientist", "analytics"],
    "flutter": ["dart"],
    "react native": ["rn", "mobile"],
    "swift": ["ios", "objective-c"],
    "kotlin": ["android"],
}

# Reverse map: synonym → canonical
SYNONYM_TO_CANONICAL: dict[str, str] = {}
for canonical, synonyms in SKILL_SYNONYMS.items():
    SYNONYM_TO_CANONICAL[canonical.lower()] = canonical
    for syn in synonyms:
        SYNONYM_TO_CANONICAL[syn.lower()] = canonical

SPECIALIZATION_KEYWORDS: dict[str, list[str]] = {
    "BACKEND": ["backend", "бэкенд", "server", "сервер", "api", "микросервис", "microservice"],
    "FRONTEND": ["frontend", "фронтенд", "front-end", "ui", "frontend", "веб-интерфейс"],
    "FULLSTACK": ["fullstack", "full-stack", "full stack", "полный стек"],
    "MOBILE": ["mobile", "мобайл", "ios", "android", "flutter", "react native"],
    "DEVOPS": ["devops", "девопс", "infrastructure", "инфраструктура", "sre", "cloud"],
    "DATA_SCIENCE": ["data science", "дата сайенс", "data scientist", "аналитик", "ml", "machine learning"],
    "QA": ["qa", "тестировщик", "quality assurance", "тестирование", "qa engineer"],
    "SECURITY": ["security", "безопасность", "infosec", "security engineer"],
    "EMBEDDED": ["embedded", "встраиваемые", "firmware", "iot"],
    "GAME_DEV": ["game dev", "game development", "игры", "gamedev", "unity", "unreal"],
    "ML_AI": ["ml", "machine learning", "ai", "искусственный интеллект", "нейросети"],
    "DESIGN_UX": ["ux", "ui design", "дизайн", "designer", "figma"],
    "MANAGEMENT": ["management", "менеджмент", "team lead", "tech lead", "руководитель"],
}

GRADE_KEYWORDS: dict[str, list[str]] = {
    "JUNIOR": ["junior", "джуниор", "начинающий", "стажер", "intern"],
    "MIDDLE": ["middle", "мидл", "средний"],
    "MIDDLE_PLUS": ["middle+", "middle plus", "сильный middle"],
    "SENIOR": ["senior", "синьор", "опытный", "advanced"],
    "LEAD": ["lead", "лид", "team lead", "tech lead", "руководитель команды"],
    "PRINCIPAL": ["principal", "принципал", "архитектор", "architect", "chief"],
}


# ─────────────────────────── Fallback TF-IDF Embedding ───────────────────────────

class TfidfFallback:
    """Lightweight fallback when transformers unavailable."""

    def __init__(self, dim: int = EMBEDDING_DIM):
        self.dim = dim
        self.vocabulary: dict[str, int] = {}
        self.idf: dict[str, float] = {}

    def fit(self, corpus: list[str]) -> None:
        """Fit vocabulary from corpus."""
        from collections import Counter

        doc_freq: Counter = Counter()
        total_docs = len(corpus)

        for doc in corpus:
            tokens = self._tokenize(doc)
            unique_tokens = set(tokens)
            for token in unique_tokens:
                doc_freq[token] += 1

        self.vocabulary = {token: idx for idx, token in enumerate(doc_freq.keys())}
        self.idf = {
            token: np.log((total_docs + 1) / (freq + 1)) + 1.0
            for token, freq in doc_freq.items()
        }

    def embed(self, text: str) -> list[float]:
        """Generate TF-IDF embedding via feature hashing."""
        tokens = self._tokenize(text)
        vector = np.zeros(self.dim, dtype=np.float32)

        for token in tokens:
            if token in self.vocabulary:
                idx = self.vocabulary[token]
            else:
                # Feature hashing
                idx = int(hashlib.md5(token.encode()).hexdigest(), 16) % self.dim

            weight = self.idf.get(token, 1.0)
            vector[idx] += weight

        # Normalize
        norm = np.linalg.norm(vector)
        if norm > 0:
            vector = vector / norm

        return vector.tolist()

    @staticmethod
    def _tokenize(text: str) -> list[str]:
        """Simple tokenization."""
        import re

        text = text.lower()
        text = re.sub(r"[^\w\s]", " ", text)
        tokens = text.split()
        return [t for t in tokens if len(t) > 1]


tfidf_fallback = TfidfFallback()


# ─────────────────────────── Skill Extraction ───────────────────────────

def extract_skills(text: str) -> tuple[list[str], list[str]]:
    """
    Extract skills from text.
    Returns (required_skills, preferred_skills).
    """
    text_lower = text.lower()
    found_skills: dict[str, int] = {}  # canonical → count

    for synonym, canonical in SYNONYM_TO_CANONICAL.items():
        if synonym in text_lower:
            found_skills[canonical] = found_skills.get(canonical, 0) + 1

    # Required: mentioned with "обязательно", "must", "required", "нужен"
    required_markers = ["обязательно", "must", "required", "нужен", "нужна", "требуется", "обязательн"]
    preferred_markers = ["желательно", "preference", "bonus", "плюс", "будет плюсом", "nice to have"]

    required: list[str] = []
    preferred: list[str] = []

    for skill in found_skills:
        # Check context around skill mention
        skill_pos = text_lower.find(skill.lower())
        if skill_pos >= 0:
            context_start = max(0, skill_pos - 100)
            context_end = min(len(text_lower), skill_pos + len(skill) + 100)
            context = text_lower[context_start:context_end]

            if any(marker in context for marker in required_markers):
                required.append(skill)
            elif any(marker in context for marker in preferred_markers):
                preferred.append(skill)
            else:
                required.append(skill)  # Default to required

    return list(set(required)), list(set(preferred))


def extract_specialization(text: str) -> Optional[str]:
    """Extract specialization from text."""
    text_lower = text.lower()
    scores: dict[str, int] = {}

    for spec, keywords in SPECIALIZATION_KEYWORDS.items():
        for keyword in keywords:
            if keyword in text_lower:
                scores[spec] = scores.get(spec, 0) + 1

    if scores:
        return max(scores, key=scores.get)
    return None


def extract_seniority(text: str) -> Optional[str]:
    """Extract seniority/grade from text."""
    text_lower = text.lower()
    scores: dict[str, int] = {}

    for grade, keywords in GRADE_KEYWORDS.items():
        for keyword in keywords:
            if keyword in text_lower:
                scores[grade] = scores.get(grade, 0) + 1

    if scores:
        return max(scores, key=scores.get)
    return None


# ─────────────────────────── Endpoints ───────────────────────────

@app.get("/health")
async def health():
    """Health check with model status."""
    return {
        "status": "ok",
        "models": {
            "embedding": {
                "ready": registry.embedding_ready,
                "name": EMBEDDING_MODEL,
                "dim": EMBEDDING_DIM,
            },
            "reranker": {
                "ready": registry.reranker_ready,
                "name": RERANKER_MODEL,
            },
            "ner": {
                "ready": registry.ner_ready,
            },
        },
        "cache": {
            "size": len(registry._embed_cache),
            "max_size": CACHE_SIZE,
        },
    }


@app.post("/embed", response_model=EmbedResponse)
async def embed(req: EmbedRequest):
    """Generate embedding for a single text."""
    t0 = time.time()

    # Check cache
    cache_key = registry.cache_key(req.text)
    cached = registry.get_cached(cache_key)
    if cached is not None:
        return EmbedResponse(
            embedding=cached,
            dim=EMBEDDING_DIM,
            provider="transformers-cached",
            cached=True,
            latency_ms=(time.time() - t0) * 1000,
        )

    # Use transformers if available
    if registry.embedding_ready and registry.embedding_model is not None:
        try:
            embedding = registry.embedding_model.encode(
                req.text,
                normalize_embeddings=True,
                convert_to_numpy=True,
            )
            result = embedding.tolist()
            registry.set_cached(cache_key, result)
            return EmbedResponse(
                embedding=result,
                dim=EMBEDDING_DIM,
                provider="transformers",
                latency_ms=(time.time() - t0) * 1000,
            )
        except Exception as e:
            logger.warning(f"Transformers embed failed: {e}, using TF-IDF fallback")

    # Fallback: TF-IDF
    result = tfidf_fallback.embed(req.text)
    return EmbedResponse(
        embedding=result,
        dim=EMBEDDING_DIM,
        provider="tfidf-fallback",
        latency_ms=(time.time() - t0) * 1000,
    )


@app.post("/embed/batch", response_model=EmbedBatchResponse)
async def embed_batch(req: EmbedBatchRequest):
    """Generate embeddings for multiple texts."""
    t0 = time.time()

    if registry.embedding_ready and registry.embedding_model is not None:
        try:
            embeddings = registry.embedding_model.encode(
                req.texts,
                normalize_embeddings=req.normalize,
                convert_to_numpy=True,
                batch_size=32,
                show_progress_bar=False,
            )
            return EmbedBatchResponse(
                embeddings=embeddings.tolist(),
                dim=EMBEDDING_DIM,
                provider="transformers",
                latency_ms=(time.time() - t0) * 1000,
            )
        except Exception as e:
            logger.warning(f"Batch embed failed: {e}, using TF-IDF fallback")

    # Fallback
    embeddings = [tfidf_fallback.embed(text) for text in req.texts]
    return EmbedBatchResponse(
        embeddings=embeddings,
        dim=EMBEDDING_DIM,
        provider="tfidf-fallback",
        latency_ms=(time.time() - t0) * 1000,
    )


@app.post("/ner", response_model=NERResponse)
async def ner(req: NERRequest):
    """Named Entity Recognition."""
    t0 = time.time()

    if registry.ner_ready and registry.spacy_nlp is not None:
        doc = registry.spacy_nlp(req.text)
        entities = [
            NEREntity(
                text=ent.text,
                label=ent.label_,
                start=ent.start_char,
                end=ent.end_char,
            )
            for ent in doc.ents
        ]
    else:
        # Simple keyword-based fallback
        entities = []
        text_lower = req.text.lower()

        for skill in SYNONYM_TO_CANONICAL:
            if skill in text_lower:
                pos = text_lower.find(skill)
                entities.append(
                    NEREntity(text=skill, label="TECH", start=pos, end=pos + len(skill))
                )

    return NERResponse(
        entities=entities,
        latency_ms=(time.time() - t0) * 1000,
    )


@app.post("/rerank", response_model=RerankResponse)
async def rerank(req: RerankRequest):
    """Cross-encoder re-ranking."""
    t0 = time.time()

    if not req.documents:
        return RerankResponse(results=[], latency_ms=0)

    if registry.reranker_ready and registry.reranker_model is not None:
        try:
            # Prepare pairs
            pairs = [(req.query, doc) for doc in req.documents]

            # Score with cross-encoder
            scores = registry.reranker_model.predict(
                pairs,
                show_progress_bar=False,
            )

            # Create results
            results = [
                RerankResult(
                    index=i,
                    score=float(scores[i]),
                    document=doc,
                )
                for i, doc in enumerate(req.documents)
            ]

            # Sort by score descending
            results.sort(key=lambda x: x.score, reverse=True)

            # Limit to top_k
            if req.top_k:
                results = results[: req.top_k]

            return RerankResponse(
                results=results,
                latency_ms=(time.time() - t0) * 1000,
            )
        except Exception as e:
            logger.warning(f"Rerank failed: {e}")

    # Fallback: return original order
    results = [
        RerankResult(index=i, score=0.5, document=doc)
        for i, doc in enumerate(req.documents[: req.top_k or len(req.documents)])
    ]
    return RerankResponse(results=results, latency_ms=(time.time() - t0) * 1000)


@app.post("/parse-query", response_model=ParseQueryResponse)
async def parse_query(req: ParseQueryRequest):
    """Full query understanding: parse + embed."""
    t0 = time.time()

    # Extract structured information
    required_skills, preferred_skills = extract_skills(req.query)
    specialization = extract_specialization(req.query)
    seniority = extract_seniority(req.query)

    parsed = ParsedQuery(
        specialization=specialization,
        seniority=seniority,
        required_skills=required_skills,
        preferred_skills=preferred_skills,
        context=req.query[:200],
        intent="hire",
    )

    # Generate embedding
    embed_start = time.time()
    embed_result = await embed(EmbedRequest(text=req.query))
    embed_latency = (time.time() - embed_start) * 1000

    return ParseQueryResponse(
        parsed=parsed,
        embedding=embed_result.embedding,
        embedding_provider=embed_result.provider,
        latency_ms=(time.time() - t0) * 1000,
    )


# ─────────────────────────── Utility Endpoints ───────────────────────────

@app.post("/fit-tfidf")
async def fit_tfidf(corpus: list[str]):
    """Fit TF-IDF vocabulary on a corpus (for fallback)."""
    tfidf_fallback.fit(corpus)
    return {
        "vocabulary_size": len(tfidf_fallback.vocabulary),
        "corpus_size": len(corpus),
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000, workers=1)