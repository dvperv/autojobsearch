#!/bin/bash

# AutoJobSearch Deployment Script v2.0
# Поддержка user-specific HH.ru API

set -e

echo "🚀 Начало развертывания AutoJobSearch MVP v2.0"
echo "📱 С поддержкой user-specific HH.ru API"
echo ""

# 1. Проверка зависимостей
echo "📋 Проверка зависимостей..."
command -v docker >/dev/null 2>&1 || { echo "❌ Docker не установлен"; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "❌ Docker Compose не установлен"; exit 1; }
command -v git >/dev/null 2>&1 || { echo "❌ Git не установлен"; exit 1; }

# 2. Клонирование репозитория
echo "📥 Клонирование репозитория..."
if [ ! -d "autojobsearch-mvp" ]; then
    git clone https://github.com/autojobsearch/autojobsearch-mvp.git
fi
cd autojobsearch-mvp

# 3. Настройка окружения
echo "⚙️ Настройка окружения..."
if [ ! -f ".env.production" ]; then
    cp .env.example .env.production

    echo ""
    echo "⚠️  ВАЖНО: Необходимо настроить HH.ru OAuth!"
    echo ""
    echo "Шаги настройки:"
    echo "1. Зарегистрируйте приложение на https://dev.hh.ru/admin"
    echo "2. Получите Client ID и Client Secret"
    echo "3. Укажите callback URL: https://ваш-домен/api/auth/hh/callback"
    echo "4. Добавьте в .env.production:"
    echo "   HH_CLIENT_ID=ваш_client_id"
    echo "   HH_CLIENT_SECRET=ваш_client_secret"
    echo "   HH_REDIRECT_URL=https://ваш-домен/api/auth/hh/callback"
    echo ""
    echo "Затем запустите скрипт снова: ./deploy.sh"
    exit 0
fi

# 4. Проверка HH.ru конфигурации
echo "🔍 Проверка HH.ru конфигурации..."
if grep -q "your_hh_client_id" .env.production; then
    echo "❌ HH.ru конфигурация не настроена!"
    echo "Пожалуйста, настройте .env.production как указано выше"
    exit 1
fi

# 5. Загрузка переменных окружения
export $(cat .env.production | grep -v '^#' | xargs)

# 6. Сборка образов
echo "🔨 Сборка Docker образов..."
docker-compose -f docker-compose.production.yml build

# 7. Запуск миграций базы данных
echo "🗄️ Запуск миграций базы данных..."
docker-compose -f docker-compose.production.yml run --rm backend \
    ./migrate -path /app/migrations -database "$DATABASE_URL" up

# 8. Создание таблиц для HH.ru токенов
echo "🔐 Создание таблиц для HH.ru токенов..."
docker-compose -f docker-compose.production.yml exec -T postgres psql -U $DB_USER -d autojobsearch <<EOF
CREATE TABLE IF NOT EXISTS hh_tokens (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    token_type VARCHAR(50),
    scope TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hh_tokens_expires_at ON hh_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_hh_tokens_user_id ON hh_tokens(user_id);
EOF

# 9. Запуск сервисов
echo "🚀 Запуск сервисов..."
docker-compose -f docker-compose.production.yml up -d

# 10. Ожидание запуска
echo "⏳ Ожидание запуска сервисов..."
sleep 30

# 11. Проверка здоровья
echo "🏥 Проверка здоровья сервисов..."
HEALTH_CHECKS=(
    "http://localhost:8080/health"
    "http://localhost:8080/api/hh/status"
    "http://localhost:9090/-/healthy"
    "http://localhost:3001/api/health"
)

for url in "${HEALTH_CHECKS[@]}"; do
    if curl -f -s --retry 3 --retry-delay 5 "$url" > /dev/null; then
        echo "✅ $url - OK"
    else
        echo "❌ $url - FAILED"
        exit 1
    fi
done

# 12. Создание первого администратора
echo "👨‍💼 Создание администратора..."
ADMIN_PASSWORD=$(openssl rand -base64 12)
docker-compose -f docker-compose.production.yml exec -T backend \
    ./create_admin --email admin@autojobsearch.com --password "$ADMIN_PASSWORD"

# 13. Настройка SSL
if [ ! -z "$DOMAIN" ]; then
    echo "🔒 Настройка SSL для $DOMAIN..."
    docker-compose -f docker-compose.production.yml run --rm certbot \
        certonly --webroot --webroot-path=/var/www/html \
        -d "$DOMAIN" -d "api.$DOMAIN" \
        --email "$ADMIN_EMAIL" --agree-tos --non-interactive --force-renewal
fi

# 14. Сборка мобильных приложений
echo "📱 Подготовка мобильных приложений..."

# Android
echo "🤖 Сборка Android APK..."
cd android
./gradlew assembleRelease
cd ..

# iOS (требуется macOS)
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "🍎 Сборка iOS IPA..."
    cd ios
    xcodebuild -workspace AutoJobSearch.xcworkspace \
               -scheme AutoJobSearch \
               -configuration Release \
               -archivePath build/AutoJobSearch.xcarchive \
               archive
    cd ..
fi

# 15. Финальный отчет
echo ""
echo "🎉 Развертывание завершено!"
echo ""
echo "📊 Доступные сервисы:"
echo "   🌐 Frontend: http://localhost"
echo "   🔧 Backend API: http://localhost:8080"
echo "   🔐 HH.ru OAuth: Настроен"
echo "   📈 Мониторинг: http://localhost:3001"
echo ""
echo "🔑 Администратор: admin@autojobsearch.com"
echo "🔐 Пароль: $ADMIN_PASSWORD"
echo ""
echo "📱 Мобильные приложения:"
echo "   Android APK: ./android/app/build/outputs/apk/release/app-release.apk"
echo "   iOS IPA: ./ios/build/AutoJobSearch.ipa"
echo ""
echo "🚨 Следующие шаги:"
echo "   1. Проверьте HH.ru OAuth: https://$DOMAIN/api/hh/status"
echo "   2. Загрузите приложения в магазины"
echo "   3. Настройте Firebase для push уведомлений"
echo "   4. Протестируйте полный цикл автоматизации"
echo ""
echo "📚 Документация: ./docs/HH_OAuth_Setup.md"
echo "🆘 Поддержка: support@autojobsearch.com"