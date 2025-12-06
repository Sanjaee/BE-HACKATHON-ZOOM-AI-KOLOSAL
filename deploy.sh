echo "🚀 [1/5] Pulling latest code from GitHub..."
git pull

echo "🧱 [2/5] Stopping backend container..."
docker compose stop backend

echo "🧹 [3/5] Removing backend container..."
docker compose down

echo "⚙️ [4/5] Rebuilding and starting backend container..."
docker compose up --build -d

echo "✅ Deploy finished successfully!"