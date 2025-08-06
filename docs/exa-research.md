# Exa Research API Integration

⚠️ **EXPENSIVE FEATURE WARNING** ⚠️

The Exa Research API is a powerful but **EXPENSIVE** feature that can cost **$5-$15+ per request**.
This feature is **DISABLED by default** and requires explicit user consent for each operation.

## What is Exa Research?

Exa Research is an advanced AI-powered research system that:
- Performs deep web research with multiple search iterations
- Synthesizes information from dozens of sources
- Generates **structured data** matching your specified JSON schema
- Provides citations and quotes for all information
- Can handle complex research tasks that would take humans hours

## Cost Structure

### Model Pricing
- **exa-research**: ~$5 per task (20-40 seconds)
- **exa-research-pro**: ~$15 per task (60-90 seconds)

### What You're Paying For
- Multiple web searches (10-50+ queries)
- Full page content extraction and analysis
- LLM reasoning and synthesis
- Structured data generation

## Configuration

### Required Environment Variables

```bash
# REQUIRED: Your Exa API key
export EXA_API_KEY="your-api-key-here"

# OPTIONAL: Maximum cost per task (default: $15)
export EXA_RESEARCH_MAX_COST="15.0"

# OPTIONAL: Disable consent requirement (NOT RECOMMENDED)
# export EXA_RESEARCH_NO_CONSENT="true"
```

## Safety Features

### 1. Consent Required
By default, **every research task requires explicit user consent** acknowledging the cost.

### 2. Cost Limits
Default maximum cost is $15 per task. Tasks exceeding this will be rejected.

### 3. Audit Trail
All research tasks are logged with:
- Timestamp
- User consent confirmation
- Estimated and actual costs
- Complete event history

### 4. Project Isolation
Research tasks are isolated per project to prevent accidental cross-project charges.

## Usage Examples

### Creating a Research Task

The system will **ALWAYS** warn you about costs before proceeding:

```bash
# This will show a cost warning and require confirmation
gismo-knowledge research "Compare the top 5 JavaScript frameworks" \
  --model exa-research \
  --consent  # Required flag to acknowledge cost
```

### With Structured Output

You can request structured data matching a specific schema:

```json
{
  "frameworks": [
    {
      "name": "string",
      "pros": ["string"],
      "cons": ["string"],
      "popularity": "number",
      "use_cases": ["string"]
    }
  ]
}
```

### Monitoring Tasks

```bash
# List active research tasks
gismo-knowledge research-tasks

# Check task status
gismo-knowledge research-status <task-id>

# Cancel a running task (may still incur partial charges)
gismo-knowledge research-cancel <task-id>
```

## SQL Access

Monitor costs and usage via SQL:

```sql
-- Total spent on research
SELECT SUM(actual_cost) as total_spent
FROM exa_research_tasks
WHERE status = 'completed';

-- Tasks by project
SELECT project_context, COUNT(*) as task_count, SUM(actual_cost) as cost
FROM exa_research_tasks
GROUP BY project_context;

-- Failed tasks (may have incurred costs)
SELECT id, instructions, error_message, actual_cost
FROM exa_research_tasks
WHERE status = 'failed';

-- Audit trail for a task
SELECT event_type, event_data, created_at
FROM exa_research_events
WHERE task_id = 'your-task-id'
ORDER BY created_at;
```

## Best Practices

### 1. Test with Regular Search First
Always try the regular Exa search (`gismo-knowledge exa`) before using Research.

### 2. Use Specific Instructions
More specific instructions lead to better results and potentially lower costs:
- ❌ "Tell me about databases"
- ✅ "Compare PostgreSQL vs MySQL for a high-traffic e-commerce site"

### 3. Define Output Schema
Structured output ensures you get exactly what you need:
```json
{
  "comparison": {
    "winner": "string",
    "reasons": ["string"],
    "score": "number"
  }
}
```

### 4. Monitor Costs
Regularly check your spending:
```bash
gismo-query "SELECT DATE(created_at) as day, SUM(actual_cost) as daily_cost FROM exa_research_tasks GROUP BY day"
```

### 5. Use Caching
Research results are automatically cached. The same research won't be repeated.

## Emergency Controls

### Disable Research Entirely
Remove the API key:
```bash
unset EXA_API_KEY
```

### Set Low Cost Limit
```bash
export EXA_RESEARCH_MAX_COST="1.0"  # Max $1 per task
```

### Force Consent for All Operations
This is the default, but ensure it's not disabled:
```bash
unset EXA_RESEARCH_NO_CONSENT  # Ensure consent is required
```

## Architecture

### Async Processing
Research tasks run asynchronously with:
- Background polling every 10 seconds
- Automatic retry on transient failures
- Timeout after 5 minutes
- Graceful cancellation support

### Database Schema
- `exa_research_tasks`: Main task records with costs
- `exa_research_events`: Complete audit trail
- `v_active_research_tasks`: Virtual view of running tasks

### Cost Tracking
Every task records:
- `estimated_cost`: Before starting
- `actual_cost`: After completion
- `user_consent`: Audit trail of consent
- `consent_timestamp`: When consent was given

## Troubleshooting

### Task Stuck in "polling" Status
Tasks timeout after 5 minutes. Check the events:
```sql
SELECT * FROM exa_research_events WHERE task_id = 'stuck-task-id';
```

### High Costs
Review your usage:
```sql
SELECT instructions, model, actual_cost
FROM exa_research_tasks
WHERE actual_cost > 10
ORDER BY actual_cost DESC;
```

### API Key Issues
Verify the key is set:
```bash
echo $EXA_API_KEY | head -c 10  # Should show first 10 chars
```

## ⚠️ Final Warning

**This feature can cost real money!** A single research task can cost $15 or more.
Always:

1. **Read the cost warning** before confirming
2. **Use regular search first** when possible
3. **Monitor your spending** regularly
4. **Set appropriate cost limits** for your budget
5. **Never disable consent requirements** in production

Remember: With great research power comes great financial responsibility!