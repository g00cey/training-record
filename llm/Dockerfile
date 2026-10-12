FROM python:3.13-slim

LABEL description="Spin Bike Image Extraction Server"

WORKDIR /app

RUN pip install --no-cache-dir pillow

COPY server.py .

EXPOSE 8646

CMD ["python3", "server.py"]
