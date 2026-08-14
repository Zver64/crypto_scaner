FROM node:24-alpine

RUN mkdir -p /workspace/frontend \
    && chown -R node:node /workspace

WORKDIR /workspace/frontend
USER node

COPY --chown=node:node package.json package-lock.json ./
RUN npm ci

COPY --chown=node:node . .

EXPOSE 3000

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
