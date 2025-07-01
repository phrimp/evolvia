# Skill Extraction Prompt for PowerPoint Content

## Task Overview

You are a skill extraction expert. Your task is to analyze PowerPoint content and identify all skills, technologies, tools, methodologies, and competencies mentioned or implied in the content. Generate a comprehensive JSON output that follows the provided Go model structure.

## Input

PowerPoint content including:

- Slide titles and text
- Bullet points and descriptions
- Technical terms and acronyms
- Job roles and responsibilities
- Project descriptions
- Technology stacks
- Methodologies and frameworks

## Instructions

### 1. Skill Identification Process

- _Read carefully_: Analyze all text content from the PowerPoint
- _Identify explicit skills_: Look for directly mentioned technologies, tools, programming languages, frameworks, methodologies
- _Identify implicit skills_: Infer skills from context, job descriptions, project requirements, or industry-specific language
- _Categorize appropriately_: Group skills into logical categories (Technology, Programming, Business, Design, etc.)
- _Consider skill levels_: When possible, infer the proficiency level mentioned or required

### 2. Pattern Recognition Guidelines

For each identified skill, create comprehensive identification patterns:

_Primary Patterns_ (high confidence indicators):

- Exact skill names: "Python", "React", "Project Management"
- Common variations: "JavaScript" vs "JS", "Machine Learning" vs "ML"
- Technical certifications: "AWS Certified", "PMP Certified"

_Secondary Patterns_ (supporting evidence):

- Related tools: "pip", "npm" (indicates Python/Node.js)
- Contextual phrases: "developed in", "experience with", "proficient in"
- Industry jargon: "agile methodology", "DevOps practices"

_Academic Patterns_ (educational context):

- Degree requirements: "Computer Science", "MBA"
- Course names: "Data Structures", "Digital Marketing"
- Academic projects: "thesis on", "research in"

_Negative Patterns_ (reduce confidence):

- Words that might cause false positives
- Context that suggests the skill is NOT being referenced

### 3. Metadata Guidelines

For each skill, provide:

- _Difficulty_: Rate 1-10 based on typical learning curve
- _Time to Learn_: Estimate hours for basic proficiency
- _Industry_: Relevant industries where this skill applies
- _Job Roles_: Common job titles that require this skill
- _Market Demand_: Rate 0.0-1.0 based on current market trends
- _Trending_: Boolean indicating if it's a trending skill

### 4. Relationship Mapping

Identify relationships between skills:

- _Prerequisites_: Skills needed before learning this one
- _Builds On_: Skills that this one extends
- _Related_: Similar or complementary skills
- _Specialization_: More specific versions of broader skills

## Output Format

Generate a JSON array where each object represents a skill following this structure:

{
"skills": [
{
"name": "Python",
"description": "High-level programming language known for its simplicity and versatility",
"identification_rules": {
"primary_patterns": [
{
"text": "Python",
"weight": 1.0,
"type": "exact",
"case_sensitive": false,
"min_word_boundary": true
},
{
"text": "python programming",
"weight": 0.9,
"type": "exact",
"case_sensitive": false,
"min_word_boundary": false
}
],
"secondary_patterns": [
{
"text": "pip install",
"weight": 0.7,
"type": "context",
"case_sensitive": false,
"min_word_boundary": false
},
{
"text": "django",
"weight": 0.6,
"type": "related",
"case_sensitive": false,
"min_word_boundary": true
}
],
"academic_patterns": [
{
"text": "python course",
"weight": 0.8,
"type": "context",
"case_sensitive": false,
"min_word_boundary": false
}
],
"negative_patterns": [
{
"text": "python snake",
"weight": -0.9,
"type": "context",
"case_sensitive": false,
"min_word_boundary": false
}
],
"min_primary_matches": 1,
"min_secondary_matches": 0,
"min_academic_matches": 0,
"min_total_score": 0.5,
"context_window": 10
},
"common_names": ["Python", "Python Programming", "Python Development"],
"abbreviations": ["Py"],
"technical_terms": ["CPython", "PyPy", "Pythonic"],
"category": {
"name": "Programming Languages",
"path": "Technology/Programming/Languages",
"level": 3
},
"tags": ["programming", "scripting", "backend", "data-science", "automation"],
"relations": [
{
"skill_name": "Programming Fundamentals",
"relation_type": "prerequisite",
"strength": 0.8,
"description": "Basic programming concepts needed before learning Python"
},
{
"skill_name": "Django",
"relation_type": "builds_on",
"strength": 0.9,
"description": "Django is a Python web framework"
}
],
"metadata": {
"industry": ["Technology", "Finance", "Healthcare", "Education"],
"job_roles": ["Software Developer", "Data Scientist", "Backend Engineer", "DevOps Engineer"],
"difficulty": 3,
"time_to_learn": 120,
"trending": true,
"market_demand": 0.9
},
"is_active": true,
"version": 1
}
],
"extraction_metadata": {
"source_type": "powerpoint",
"total_skills_found": 25,
"confidence_threshold": 0.6,
"extraction_date": "2025-07-01T00:00:00Z",
"processing_notes": "Extracted from slides covering full-stack development training program"
}
}

## Quality Guidelines

1. _Completeness_: Don't miss obvious skills mentioned in the content
2. _Accuracy_: Ensure skill names and descriptions are correct
3. _Relevance_: Focus on skills that are actually relevant to the content context
4. _Consistency_: Use standard skill names and avoid duplicates
5. _Context Awareness_: Consider the industry/domain context of the PowerPoint
6. _Granularity_: Balance between being specific and avoiding over-fragmentation

## Pattern Weight Guidelines

- _1.0_: Perfect exact match (skill name exactly as written)
- _0.8-0.9_: Very strong indicators (common variations, clear context)
- _0.6-0.7_: Good supporting evidence (related tools, contextual phrases)
- _0.4-0.5_: Weak indicators (might need multiple matches)
- _0.0 to -1.0_: Negative patterns (false positives, wrong context)

## Common Skill Categories to Look For

- _Programming Languages_: Python, JavaScript, Java, C++, etc.
- _Frameworks & Libraries_: React, Django, Spring, etc.
- _Databases_: MySQL, MongoDB, PostgreSQL, etc.
- _Cloud Platforms_: AWS, Azure, Google Cloud, etc.
- _Tools & Software_: Git, Docker, Kubernetes, etc.
- _Methodologies_: Agile, Scrum, DevOps, etc.
- _Business Skills_: Project Management, Leadership, Communication, etc.
- _Design Skills_: UI/UX, Graphic Design, Prototyping, etc.
- _Data Skills_: Analytics, Machine Learning, Statistics, etc.

Now analyze the provided PowerPoint content and generate the comprehensive skill extraction JSON following this format.
