# FSP ML Service

Python ML service for semantic matching V2.

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check + model status |
| `/embed` | POST | Generate single embedding (768-dim) |
| `/embed/batch` | POST | Batch embeddings |
| `/ner` | POST | Named Entity Recognition |
| `/rerank` | POST | Cross-encoder re-ranking |
| `/parse-query` | POST | Full query understanding |
| `/fit-tfidf` | POST | Fit TF-IDF fallback |

## Models

- **Embedding**: `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` (768-dim, multilingual)
- **Re-ranker**: `cross-encoder/ms-marco-MiniLM-L-6-v2`
- **NER**: `spaCy ru_core_news_sm`

## Quick Start

```bash
# Install dependencies
pip install -r requirements.txt

# Run
uvicorn main:app --host 0.0.0.0 --port 8000

# Or with Docker
docker build -t fsp-ml-service .
docker run -p 8000:8000 fsp-ml-service
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL_NAME` | `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` | Embedding model |
| `RERANKER_NAME` | `cross-encoder/ms-marco-MiniLM-L-6-v2` | Re-ranker model |
| `CACHE_SIZE` | `2000` | Embedding cache size |
| `REDIS_URL` | `None` | Redis URL (optional) |

## API Examples

### Health Check
```bash
curl http://localhost:8000/health
```

### Generate Embedding
```bash
curl -X POST http://localhost:8000/embed \
  -H "Content-Type: application/json" \
  -d '{"text": "Senior backend developer with Go and Kubernetes"}'
```

### Re-ranking
```bash
curl -X POST http://localhost:8000/rerank \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Backend developer with Go experience",
    "documents": [
      "Senior Go developer with 5 years experience",
      "React frontend developer",
      "DevOps engineer with Kubernetes"
    ],
    "top_k": 2
  }'
```

### Parse Query
```bash
curl -X POST http://localhost:8000/parse-query \
  -H "Content-Type: application/json" \
  -d '{"query": "Ищем senior backend разработчика на Go с опытом в микросервисах"}'
```

## Docker

```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
# Pre-download models
RUN python -c "from sentence_transformers import SentenceTransformer; \
    SentenceTransformer('sentence-transformers/paraphrase-multilingual-mpnet-base-v2'); \
    SentenceTransformer('cross-encoder/ms-marco-MiniLM-L-6-v2')"
COPY . .
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```