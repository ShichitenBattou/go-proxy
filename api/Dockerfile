FROM python:3.14.3

WORKDIR /app

COPY . /app/

ENTRYPOINT [ "python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8081" ]