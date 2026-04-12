#!/bin/bash

# TechInsight API Monitoring Setup Script
echo "🔧 Starting TechInsight API Monitoring Stack..."

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null
then
    echo "❌ docker-compose could not be found. Please install Docker Compose."
    exit 1
fi




# Create necessary directories
echo "📁 Creating monitoring directories..."
mkdir -p configs/prometheus/rules
mkdir -p configs/grafana/dashboards
mkdir -p configs/grafana/provisioning/dashboards
mkdir -p configs/grafana/provisioning/datasources

# Start monitoring services
echo "🚀 Starting monitoring services..."
docker-compose -f docker-compose.monitor.yml up -d

# Wait for services to start
echo "⏳ Waiting for services to be healthy..."
sleep 30

# Check service health
echo "🔍 Checking service health..."

# Check Prometheus
if curl -s http://localhost:9090/-/healthy > /dev/null; then
    echo "✅ Prometheus is healthy (http://localhost:9090)"
else
    echo "❌ Prometheus is not responding"
fi

# Check Grafana
if curl -s http://localhost:3000/api/health > /dev/null; then
    echo "✅ Grafana is healthy (http://localhost:3000)"
    echo "   Default credentials: admin/admin123"
else
    echo "❌ Grafana is not responding"
fi

# Check Elasticsearch
if curl -s http://localhost:9200/_cluster/health > /dev/null; then
    echo "✅ Elasticsearch is healthy (http://localhost:9200)"
else
    echo "❌ Elasticsearch is not responding"
fi

# Check OpenTelemetry Collector
if curl -s http://localhost:13133/ > /dev/null; then
    echo "✅ OpenTelemetry Collector is healthy"
else
    echo "❌ OpenTelemetry Collector is not responding"
fi

echo ""
echo "🎉 Monitoring stack setup complete!"
echo ""
echo "📊 Access URLs:"
echo "   • Grafana:     http://localhost:3000 (admin/admin123)"
echo "   • Prometheus:  http://localhost:9090"
echo "   • Kibana:      http://localhost:5601"
echo "   • API Metrics: http://localhost:8080/metrics"
echo ""
echo "📈 Available Dashboards:"
echo "   • Application Dashboard: http://localhost:3000/d/techinsight-app"
echo "   • Infrastructure Dashboard: http://localhost:3000/d/techinsight-infra"
echo "   • Business Metrics Dashboard: http://localhost:3000/d/techinsight-business"
echo ""
echo "⚠️  Make sure your TechInsight API is running on port 8080 for metrics collection!"
echo ""
echo "📚 Next steps:"
echo "   1. Start your API: go run cmd/api/main.go"
echo "   2. Visit Grafana and explore the dashboards"
echo "   3. Check Prometheus targets: http://localhost:9090/targets"
echo "   4. Set up alerting in Grafana if needed"
