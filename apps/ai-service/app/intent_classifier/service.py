import re
import json
from enum import Enum
from pydantic import BaseModel
from app.providers import LLMProvider
from app.providers.base import Prompt


class Intent(str, Enum):
    GREETING = "GREETING"
    SMALL_TALK = "SMALL_TALK"
    GENERAL_CHAT = "GENERAL_CHAT"
    KNOWLEDGE_QUERY = "KNOWLEDGE_QUERY"
    MEMORY_QUERY = "MEMORY_QUERY"


class IntentClassificationResult(BaseModel):
    intent: Intent
    confidence: float
    reasoning: str
    used_llm: bool


class IntentClassifierService:
    def __init__(self, llm_provider: LLMProvider):
        self.llm = llm_provider

        # Rule-based regexes for ultra-fast matching
        self.greeting_pattern = re.compile(
            r"^(hi|hello|hey|greetings|good morning|good afternoon|good evening|yo)\b[!?.]*$",
            re.IGNORECASE,
        )
        self.smalltalk_pattern = re.compile(
            r"^(how are you|who are you|what can you do|how is it going|thanks|thank you|bye|see ya)\b[!?.]*$",
            re.IGNORECASE,
        )
        self.memory_pattern = re.compile(
            r"\b(my name|what is my|who am i|my last project|remember|what did i tell you|my preference)\b",
            re.IGNORECASE,
        )

    async def classify(self, query: str, has_documents: bool = False) -> IntentClassificationResult:
        clean_q = query.strip()

        if self.greeting_pattern.match(clean_q):
            return IntentClassificationResult(intent=Intent.GREETING, confidence=1.0, reasoning="Matched greeting rule", used_llm=False)

        if self.smalltalk_pattern.match(clean_q):
            return IntentClassificationResult(intent=Intent.SMALL_TALK, confidence=1.0, reasoning="Matched smalltalk rule", used_llm=False)

        if self.memory_pattern.search(clean_q):
            return IntentClassificationResult(intent=Intent.MEMORY_QUERY, confidence=0.95, reasoning="Matched memory keyword rule", used_llm=False)

        if has_documents:
            return IntentClassificationResult(intent=Intent.KNOWLEDGE_QUERY, confidence=0.85, reasoning="Conversation has attached documents", used_llm=False)

        # Fallback to LLM Classifier
        prompt_text = f"""Classify the intent of this user query into one of: GREETING, SMALL_TALK, GENERAL_CHAT, KNOWLEDGE_QUERY, MEMORY_QUERY.
Query: "{clean_q}"
Return JSON: {{"intent": "ENUM_VALUE", "confidence": 0.9, "reasoning": "why"}}
"""
        try:
            res = await self.llm.generate(Prompt(text=prompt_text, temperature=0.0, max_tokens=100))
            data = json.loads(res.text.strip())
            return IntentClassificationResult(
                intent=Intent(data.get("intent", "GENERAL_CHAT")),
                confidence=float(data.get("confidence", 0.8)),
                reasoning=data.get("reasoning", "LLM classifier"),
                used_llm=True,
            )
        except Exception:
            return IntentClassificationResult(intent=Intent.GENERAL_CHAT, confidence=0.5, reasoning="Fallback default", used_llm=True)
