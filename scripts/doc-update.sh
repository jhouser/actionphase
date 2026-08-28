#!/bin/bash

# Documentation Auto-Update Script
# Automatically updates test counts and other metrics in documentation
# Usage: ./scripts/doc-update.sh

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔄 ActionPhase Documentation Auto-Update"
echo "========================================="
echo ""

# Track updates
UPDATES=0

# Function to update a value in a file
update_value() {
    local file=$1
    local pattern=$2
    local new_value=$3
    local description=$4

    if [ -f "$file" ]; then
        if grep -q "$pattern" "$file"; then
            # Create backup
            cp "$file" "$file.bak"

            # Perform replacement
            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS
                sed -i '' "s/$pattern/$new_value/g" "$file"
            else
                # Linux
                sed -i "s/$pattern/$new_value/g" "$file"
            fi

            echo -e "${GREEN}✓${NC} Updated $description in $file"
            ((UPDATES++))

            # Remove backup if successful
            rm "$file.bak"
        fi
    fi
}

# Function to update date
update_date() {
    local file=$1
    local today=$(date +"%B %d, %Y")

    if [ -f "$file" ]; then
        if grep -q "Last Updated:" "$file"; then
            update_value "$file" "Last Updated:.*" "Last Updated: $today" "Last Updated date"
        fi
    fi
}

# 1. Count actual test metrics
echo "1. Gathering test metrics..."
echo "----------------------------"

# Backend tests
BACKEND_TEST_FILES=$(find backend -name "*_test.go" -type f | wc -l | tr -d ' ')
BACKEND_TEST_FUNCS=$(grep -r "^func Test" backend --include="*_test.go" 2>/dev/null | wc -l | tr -d ' ')

# Frontend tests
FRONTEND_TEST_FILES=$(find frontend/src -name "*.test.tsx" -o -name "*.test.ts" -type f | wc -l | tr -d ' ')
FRONTEND_TEST_SUITES=$(grep -r "describe\(" frontend/src --include="*.test.ts*" 2>/dev/null | wc -l | tr -d ' ')
FRONTEND_TEST_CASES=$(grep -r "\(test\|it\)\(" frontend/src --include="*.test.ts*" 2>/dev/null | wc -l | tr -d ' ')

# E2E tests
E2E_TEST_FILES=$(find frontend/e2e -name "*.spec.ts" -type f | wc -l | tr -d ' ')
E2E_TEST_CASES=$(grep -r "test\(" frontend/e2e --include="*.spec.ts" 2>/dev/null | wc -l | tr -d ' ')

# Calculate totals
TOTAL_TESTS=$((BACKEND_TEST_FUNCS + FRONTEND_TEST_CASES + E2E_TEST_CASES))

echo "Backend: $BACKEND_TEST_FUNCS tests in $BACKEND_TEST_FILES files"
echo "Frontend: $FRONTEND_TEST_CASES tests in $FRONTEND_TEST_FILES files"
echo "E2E: $E2E_TEST_CASES tests in $E2E_TEST_FILES files"
echo "Total: $TOTAL_TESTS tests"
echo ""

# 2. Update TESTING.md
echo "2. Updating test documentation..."
echo "---------------------------------"

TESTING_FILE=".claude/context/TESTING.md"
if [ -f "$TESTING_FILE" ]; then
    # Update backend test count
    update_value "$TESTING_FILE" \
        "Backend:.*[0-9]\+ tests" \
        "Backend: $BACKEND_TEST_FUNCS tests" \
        "backend test count"

    # Update frontend test count
    update_value "$TESTING_FILE" \
        "Frontend:.*[0-9]\+ tests" \
        "Frontend: $FRONTEND_TEST_CASES tests" \
        "frontend test count"

    # Update E2E test count
    update_value "$TESTING_FILE" \
        "E2E:.*[0-9]\+ tests" \
        "E2E: $E2E_TEST_CASES tests" \
        "E2E test count"

    # Update total
    update_value "$TESTING_FILE" \
        "Total:.*[0-9]\+ tests" \
        "Total: $TOTAL_TESTS tests" \
        "total test count"

    # Update Last Verified date
    update_date "$TESTING_FILE"
fi

# 3. Count and update other metrics
echo ""
echo "3. Updating code metrics..."
echo "---------------------------"

# Count Go files
GO_FILES=$(find backend -name "*.go" -not -name "*_test.go" -not -path "*/vendor/*" -type f | wc -l | tr -d ' ')
GO_LINES=$(find backend -name "*.go" -not -name "*_test.go" -not -path "*/vendor/*" -type f -exec wc -l {} + | tail -1 | awk '{print $1}')

# Count TypeScript files
TS_FILES=$(find frontend/src -name "*.ts" -o -name "*.tsx" -not -name "*.test.*" -not -path "*/node_modules/*" -type f | wc -l | tr -d ' ')
TS_LINES=$(find frontend/src \( -name "*.ts" -o -name "*.tsx" \) -not -name "*.test.*" -not -path "*/node_modules/*" -type f -exec wc -l {} + | tail -1 | awk '{print $1}')

echo "Go: $GO_FILES files, ~$GO_LINES lines"
echo "TypeScript: $TS_FILES files, ~$TS_LINES lines"
echo ""

# 4. Update port references if needed
echo "4. Checking configuration consistency..."
echo "----------------------------------------"

# Get actual port from .env
if [ -f ".env" ]; then
    BACKEND_PORT=$(grep "^PORT=" .env | cut -d= -f2 || echo "3000")
    FRONTEND_PORT=$(grep "^VITE_PORT=" frontend/.env 2>/dev/null | cut -d= -f2 || echo "5173")

    echo "Backend port: $BACKEND_PORT"
    echo "Frontend port: $FRONTEND_PORT"
fi
echo ""

# 5. Generate metrics file
echo "5. Generating metrics file..."
echo "-----------------------------"

METRICS_FILE="docs/METRICS.md"
cat > "$METRICS_FILE" << EOF
# ActionPhase Metrics

**Auto-generated on**: $(date +"%B %d, %Y at %H:%M")

## Codebase Statistics

### Code Volume
- **Backend (Go)**: $GO_FILES files, ~$(echo "scale=1; $GO_LINES/1000" | bc) KLOC
- **Frontend (TypeScript)**: $TS_FILES files, ~$(echo "scale=1; $TS_LINES/1000" | bc) KLOC

### Test Coverage
- **Backend Tests**: $BACKEND_TEST_FUNCS tests across $BACKEND_TEST_FILES files
- **Frontend Tests**: $FRONTEND_TEST_CASES tests across $FRONTEND_TEST_FILES files
- **E2E Tests**: $E2E_TEST_CASES tests across $E2E_TEST_FILES files
- **Total Tests**: $TOTAL_TESTS

### Test Density
- **Backend**: $(echo "scale=2; $BACKEND_TEST_FUNCS*1000/$GO_LINES" | bc) tests per KLOC
- **Frontend**: $(echo "scale=2; $FRONTEND_TEST_CASES*1000/$TS_LINES" | bc) tests per KLOC

## Repository Structure

### Documentation Files
- **Markdown files**: $(find . -name "*.md" -not -path "*/node_modules/*" -not -path "*/.git/*" | wc -l | tr -d ' ')
- **ADRs**: $(find docs/adrs -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
- **Context files**: $(find .claude/context -name "*.md" 2>/dev/null | wc -l | tr -d ' ')

### Database
- **Migration files**: $(find backend/pkg/db/migrations -name "*.sql" 2>/dev/null | wc -l | tr -d ' ')
- **Query files**: $(find backend/pkg/db/queries -name "*.sql" 2>/dev/null | wc -l | tr -d ' ')

---
*Automatically generated by scripts/doc-update.sh*
EOF

echo -e "${GREEN}✓${NC} Created $METRICS_FILE"
((UPDATES++))
echo ""

# Summary
echo "========================================="
echo "📊 Documentation Update Summary"
echo "========================================="

if [ $UPDATES -gt 0 ]; then
    echo -e "${GREEN}✅ Successfully made $UPDATES updates${NC}"
    echo ""
    echo "Updated files:"
    echo "  • Test counts in documentation"
    echo "  • Last Updated dates"
    echo "  • Generated metrics file"
    echo ""
    echo "Run 'git diff' to review changes"
else
    echo -e "${YELLOW}⚠️  No updates were needed${NC}"
fi

exit 0
