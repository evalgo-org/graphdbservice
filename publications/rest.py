import uvicorn
from fastapi import FastAPI

app = FastAPI()


@app.get("/")
async def root():
    return "OK"

if __name__ == "__main__":
    # my_logconfig = logging.config.dictConfig(LOGGING)
    uvicorn.run("rest:app", host="0.0.0.0", port=8000, log_level="debug", reload=False)
