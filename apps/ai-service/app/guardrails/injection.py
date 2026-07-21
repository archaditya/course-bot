from app.providers import GuardrailProvider, InjectionResult


class InjectionChecker:
    def __init__(self, provider: GuardrailProvider):
        self.provider = provider
    
    async def check(self, text: str) -> InjectionResult:
        """Check text for prompt injection."""
        return await self.provider.check_injection(text)