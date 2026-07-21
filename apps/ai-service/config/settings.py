from pydantic_settings import BaseSettings
from typing import Literal


class Settings(BaseSettings):
    # API
    api_host: str = "0.0.0.0"
    api_port: int = 8000
    
    # OpenAI
    openai_api_key: str = "your_openai_api_key_here"
    openai_embedding_model: str = "text-embedding-3-small"
    openai_llm_large_model: str = "gpt-4o"
    openai_llm_mini_model: str = "gpt-4o-mini"
    
    # Qdrant
    qdrant_url: str = "http://localhost:6333"
    qdrant_api_key: str | None = None
    
    # Feature Flags
    guardrails_enabled: bool = True
    evaluator_enabled: bool = True
    max_retries: int = 3
    evaluator_threshold: float = 7.0
    
    # Provider Selection
    llm_provider: Literal["openai"] = "openai"
    embedding_provider: Literal["openai"] = "openai"
    reranker_provider: Literal["openai"] = "openai"
    guardrail_provider: Literal["openai"] = "openai"
    
    class Config:
        env_file = ".env"
        case_sensitive = False


settings = Settings()