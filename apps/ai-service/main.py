import uvicorn
from config.settings import settings


def main():
    uvicorn.run(
        "api.server:app",
        host=settings.api_host,
        port=settings.api_port,
        reload=True,
    )


if __name__ == "__main__":
    main()