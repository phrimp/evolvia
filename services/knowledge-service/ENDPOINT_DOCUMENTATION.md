# Top User Skill Progress Endpoint

## Overview
This endpoint retrieves the top 5 user skill progress with comprehensive details including skill information, progress scores, and latest verification history.

## Endpoint
```
GET /protected/user-skills/user/{userId}/top-progress
```

## Authentication
- Requires valid authentication token
- User can only access their own progress (owner permission required)
- Admins/Managers can access any user's progress

## Parameters

### Path Parameters
- `userId` (required): MongoDB ObjectID of the user

### Query Parameters
- `criteria` (optional): Sorting criteria for results
  - `overall_progress` (default): Sort by overall progress score (highest first)
  - `recent_activity`: Sort by most recent activity (last_used or updated_at)
  - `improvement`: Sort by recent improvement in verification history
- `limit` (optional): Number of results to return (1-20, default: 5)

## Request Example
```bash
GET /protected/user-skills/user/507f1f77bcf86cd799439011/top-progress?criteria=overall_progress&limit=5
```

## Response Format

### Success Response (200 OK)
```json
{
  "data": {
    "user_id": "507f1f77bcf86cd799439011",
    "skills": [
      {
        "skill_id": "507f1f77bcf86cd799439012",
        "skill_name": "JavaScript",
        "skill_description": "JavaScript programming language",
        "skill_level": "advanced",
        "progress": 85.5,
        "blooms_assessment": {
          "remember": 90.0,
          "understand": 85.0,
          "apply": 88.0,
          "analyze": 82.0,
          "evaluate": 80.0,
          "create": 78.0,
          "verified": true,
          "last_updated": "2024-01-15T10:30:00Z"
        },
        "latest_history": {
          "timestamp": "2024-01-15T10:30:00Z",
          "total_hours": 25.5,
          "trigger_event": "verification",
          "overall_score": 85.5,
          "previous_score": 80.0,
          "improvement": 5.5,
          "blooms_snapshot": {
            "remember": 90.0,
            "understand": 85.0,
            "apply": 88.0,
            "analyze": 82.0,
            "evaluate": 80.0,
            "create": 78.0,
            "verified": true,
            "last_updated": "2024-01-15T10:30:00Z"
          }
        },
        "confidence": 0.9,
        "years_experience": 3,
        "verified": true,
        "last_used": "2024-01-14T15:20:00Z",
        "updated_at": "2024-01-15T10:30:00Z"
      }
    ],
    "count": 5,
    "criteria": "overall_progress"
  },
  "message": "Top user skill progress retrieved successfully"
}
```

### Error Responses

#### 400 Bad Request
```json
{
  "error": "User ID is required"
}
```

```json
{
  "error": "Invalid user ID format"
}
```

#### 403 Forbidden
```json
{
  "error": "Insufficient permissions"
}
```

#### 404 Not Found
```json
{
  "error": "User not found"
}
```

#### 500 Internal Server Error
```json
{
  "error": "Failed to retrieve user skill progress"
}
```

## Response Fields

### UserSkillProgressDetail
- `skill_id`: MongoDB ObjectID of the skill
- `skill_name`: Display name of the skill
- `skill_description`: Detailed description of the skill
- `skill_level`: User's proficiency level (beginner, intermediate, advanced, expert)
- `progress`: Overall progress score (0-100) calculated using Bloom's taxonomy
- `blooms_assessment`: Detailed assessment across cognitive levels
- `latest_history`: Most recent verification history entry (optional)
- `confidence`: User's self-reported confidence (0-1)
- `years_experience`: Years of experience with this skill
- `verified`: Whether the skill has been verified by an authority
- `last_used`: When the skill was last used (optional)
- `updated_at`: Last modification timestamp

### ProgressHistorySummary
- `timestamp`: When the verification was recorded
- `total_hours`: Hours spent on skill development
- `trigger_event`: What triggered this verification (e.g., "verification", "self-assessment")
- `overall_score`: Overall skill score at this point in time
- `previous_score`: Previous overall score (for improvement calculation)
- `improvement`: Change from previous score
- `blooms_snapshot`: Bloom's taxonomy scores at this timestamp

## Business Logic

### Progress Calculation
1. **Hybrid Assessment**: Uses verification history when available, falls back to self-assessment
2. **Aggregated Skills**: For skills with "builds_on" relationships, calculates weighted progress from prerequisite skills
3. **Bloom's Taxonomy**: Applies weighted scoring across six cognitive levels (Remember, Understand, Apply, Analyze, Evaluate, Create)

### Sorting Criteria
- **overall_progress**: Ranks skills by calculated progress score (most advanced first)
- **recent_activity**: Prioritizes recently used or updated skills
- **improvement**: Highlights skills showing recent progress gains

### Security
- Owner permission validation ensures users can only access their own data
- Admin/Manager roles can access any user's progress
- All endpoints require authentication

## Integration Notes
- Supports existing skill verification history system
- Compatible with Bloom's taxonomy assessment framework
- Integrates with skill relationship system (prerequisite/builds_on)
- Works with both self-assessed and verified skills