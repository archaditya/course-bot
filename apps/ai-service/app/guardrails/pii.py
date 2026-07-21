from app.providers import GuardrailProvider, PIIResult


class PIIChecker:
    def __init__(self, provider: GuardrailProvider):
        self.provider = provider
    
    async def check(self, text: str) -> PIIResult:
        """Check text for PII."""
        return await self.provider.check_pii(text)