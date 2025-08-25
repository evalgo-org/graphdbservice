FROM python:3.12-slim-bookworm

RUN mkdir /app
  
# Install uv
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates git
ADD https://astral.sh/uv/0.7.5/install.sh /uv-installer.sh
RUN sh /uv-installer.sh && rm /uv-installer.sh && ls -la /root/.local/bin
ENV PATH="/root/.local/bin:$PATH"

# Set up the in-tree virtual environment
WORKDIR /app

# Copy your application code to the container
ADD . /app

# Set up pypi requied auth information
#COPY pip.conf /app/.venv/pip.conf
COPY .netrc /root/.netrc

# run uv sync
RUN pwd && ls -l
#RUN uv add "faststream[rabbit]"
RUN uv sync --index https://cache.hz.pantopix.net/repository/pypi-private/simple/

# Set the working directory
ENV PATH="/app/.venv/bin:$PATH"
 
# Copy your application code to the container
#COPY . /app
COPY *.env /app/.env
COPY *.consumer.json /app/publications.consumer.json

ENTRYPOINT [".venv/bin/python"]
CMD ["publications.py"]
