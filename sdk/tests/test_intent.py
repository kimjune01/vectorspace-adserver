"""Integration tests: conversation → intent extraction → ad matching.

4 verticals × (1 generalist + 3 specialists) = 16 advertisers.
- Specialist tests: specific conversations route to the right specialist.
- Generalist tests: vague/broad conversations route to the generalist.

Requires:
  - ANTHROPIC_API_KEY env var
  - Running sidecar (port 8081) and ad server (port 8080)

Run:
  uv run --extra test pytest tests/test_intent.py -v -s
"""

import os

import anthropic
import pytest

from vectorspace import VectorSpaceClient, INTENT_PROMPT

ADSERVER_URL = os.getenv("ADSERVER_URL", "http://localhost:8080")

# ── Marks & Fixtures ─────────────────────────────────────────────

requires_api_key = pytest.mark.skipif(
    not os.getenv("ANTHROPIC_API_KEY"),
    reason="ANTHROPIC_API_KEY not set",
)

requires_server = pytest.mark.skipif(
    os.getenv("SKIP_INTEGRATION", "0") == "1",
    reason="SKIP_INTEGRATION=1",
)


@pytest.fixture(scope="module")
def llm():
    return anthropic.Anthropic()


@pytest.fixture(scope="module")
def client():
    c = VectorSpaceClient(ADSERVER_URL)
    yield c
    c.close()


# ── Verticals ────────────────────────────────────────────────────
#
# Each vertical has 1 generalist and 3 specialists.
# Specialists bid the same price as the generalist so the auction
# is decided purely by semantic proximity, not by price.
#
# The "framing" column shows how the advertiser's intent text is
# written to carve out a niche in embedding space.

VERTICALS = {
    "physical_therapy": {
        "advertisers": [
            # (name, intent framing, bid)
            ("AllMotion PT",
             "physical therapy and movement rehabilitation for pain relief and mobility improvement",
             2.50),
            ("Peak Performance PT",
             "sports injury rehab and return-to-play training for competitive athletes",
             2.50),
            ("Gentle Care PT",
             "in-home geriatric physical therapy for elderly seniors recovering from hip and knee replacement",
             2.50),
            ("TinySteps Pediatrics",
             "pediatric occupational and physical therapy for children with developmental delays",
             2.50),
        ],
        "conversations": {
            "marathon_knee": {
                "conversation": """\
User: my knee has been killing me on long runs. outside of the knee, gets worse going downhill. marathon is in 6 weeks and i'm freaking out
Bot: That sounds like it could be your IT band. How far are you running per week?
User: like 40-50 miles. i've just been icing it and hoping it goes away but it's not
Bot: Icing helps with inflammation but probably won't fix the underlying issue at that mileage. A physical therapist could figure out what's causing it and get you to the start line.
User: yeah i think that's the move. i just don't want someone who's gonna tell me to stop running for 3 months""",
                "expected_winner": "Peak Performance PT",
            },
            "grandma_hip": {
                "conversation": """\
User: my grandmother had hip replacement surgery last week and i'm worried about her. she's 82 and lives by herself
Bot: That's a lot to manage alone. Is she doing her physical therapy exercises?
User: she's supposed to but she's scared to move. i go over when i can but i work full time. she's really frail and the rehab place discharged her way too early imo
Bot: Some physical therapists do home visits, which might be less overwhelming for her than going to a clinic.
User: that would be perfect honestly. she gets anxious leaving the house even on a good day""",
                "expected_winner": "Gentle Care PT",
            },
            "toddler_not_walking": {
                "conversation": """\
User: our son is 18 months and still not walking. the pediatrician said we should get him evaluated
Bot: Some kids just take a bit longer. Is he pulling up to stand?
User: yeah he pulls up on furniture but won't let go. he's also kind of behind on other stuff, like he can't really hold a spoon or stack blocks. idk if that's related
Bot: It could be. Those are fine motor skills, so it might be worth looking at the whole picture.
User: yeah that's what the pediatrician said too. we just want to get him some help while he's still little""",
                "expected_winner": "TinySteps Pediatrics",
            },
        },
    },
    "legal": {
        "advertisers": [
            ("CityWide Legal",
             "general legal consultation and representation for individuals and small businesses",
             2.50),
            ("CrashClaim Attorneys",
             "car accident and personal injury lawsuit representation for victims seeking compensation",
             2.50),
            ("FreshStart Family Law",
             "divorce mediation and child custody negotiation for separating parents",
             2.50),
            ("NewPath Immigration",
             "work visa applications and green card sponsorship for immigrants and employers",
             2.50),
        ],
        "conversations": {
            "rear_ended": {
                "conversation": """\
User: got rear-ended on the freeway yesterday. other driver was on their phone. went to the ER, they said whiplash and a bruised rib
Bot: I'm sorry to hear that. Have you been in contact with the other driver's insurance?
User: yeah they already called and offered me $2,000 to settle. my ER bill alone was more than that. feels like they're trying to lowball me before i figure out what's going on
Bot: That's pretty common. You don't have to accept anything right away.
User: i know but i've never dealt with anything like this. i don't even know what i'm entitled to""",
                "expected_winner": "CrashClaim Attorneys",
            },
            "divorce_with_kids": {
                "conversation": """\
User: my wife and i are separating. 12 years, two kids. this sucks
Bot: I'm sorry. Are you at a point where you're figuring out logistics?
User: trying to. we both want 50/50 custody but can't agree on the schedule. and she wants to move to a different school district which would mess everything up
Bot: Those are big decisions. Have you two been able to talk about it directly or is it too tense?
User: it's tense but not hostile. i'd rather work it out between us than go to court if we can. i just don't know how to make that happen""",
                "expected_winner": "FreshStart Family Law",
            },
            "h1b_visa": {
                "conversation": """\
User: i just got laid off and i'm on an H-1B. i have 60 days to find a new sponsor or i have to leave the country
Bot: That's an incredibly tight timeline. Do you have any job prospects?
User: i have two offers actually but neither company has ever sponsored an H-1B before. they're willing but they don't know how it works
Bot: The transfer process can be done, but it helps to have someone who knows the paperwork guide the employer through it.
User: yeah that's what i need. and honestly i'm also just tired of doing this every few years, i want to figure out the green card path too""",
                "expected_winner": "NewPath Immigration",
            },
        },
    },
    "financial": {
        "advertisers": [
            ("PlanAhead Financial",
             "comprehensive financial planning and investment advice for individuals and families",
             2.50),
            ("LaunchFund Capital",
             "startup fundraising strategy and venture capital pitch coaching for early-stage founders",
             2.50),
            ("GoldenYears Advisors",
             "retirement planning and 401k rollover optimization for pre-retirees",
             2.50),
            ("DebtFree Solutions",
             "student loan refinancing and debt repayment strategy for recent graduates",
             2.50),
        ],
        "conversations": {
            "seed_round": {
                "conversation": """\
User: i'm building a SaaS tool for restaurant inventory management. we've got about 15 paying customers and 8k in monthly revenue
Bot: Nice, that's solid early traction. What's your plan for growth?
User: i want to raise money. like a seed round, maybe 1-2 million? but i have no idea how any of that works. never talked to a VC in my life
Bot: There's a lot to figure out — how much to raise, what terms to offer, how to pitch it.
User: yeah exactly. i've been googling SAFE notes and pre-money valuation and my head is spinning. i need someone who can actually walk me through this""",
                "expected_winner": "LaunchFund Capital",
            },
            "approaching_retirement": {
                "conversation": """\
User: i'm 58 and i want to stop working by 62. i have about 1.2 million spread across a 401k and some IRAs but i honestly don't know if that's enough
Bot: It depends on a lot of factors. What are your biggest expenses?
User: house is paid off so that's good. but healthcare terrifies me. i won't have medicare until 65 and my wife is 55 so she's even further out
Bot: Yeah the gap between retirement and Medicare is one of the trickiest parts to plan for.
User: that's the thing, i've been saving for 30 years but i have no idea how to actually start spending it without running out""",
                "expected_winner": "GoldenYears Advisors",
            },
            "drowning_in_loans": {
                "conversation": """\
User: i graduated law school with 180k in student loans and i'm drowning. making minimum payments and the balance just keeps going up
Bot: That's rough. What's your income like?
User: 65k at a small firm. the loans are a mix of rates, some at 5 some at 7.5. i'm on income-driven repayment but it feels like i'm just treading water
Bot: There are a few different strategies depending on your situation. Refinancing, public service forgiveness, aggressive payoff — they all have tradeoffs.
User: yeah i've heard of all of those but i don't know which one actually makes sense for me. i just need someone to look at my numbers and tell me what to do""",
                "expected_winner": "DebtFree Solutions",
            },
        },
    },
    "home_services": {
        "advertisers": [
            ("HandyPro Services",
             "general home repair and maintenance services for homeowners",
             2.50),
            ("DreamKitchen Remodels",
             "kitchen renovation and custom cabinetry design for homeowners upgrading their kitchen",
             2.50),
            ("StormShield Roofing",
             "emergency roof repair and storm damage restoration for homeowners with leaks",
             2.50),
            ("FlowFix Plumbing",
             "emergency plumbing and sewer line repair for homeowners with water damage or flooding",
             2.50),
        ],
        "conversations": {
            "kitchen_remodel": {
                "conversation": """\
User: we bought a 1970s ranch house and the kitchen is just brutal. laminate counters, harvest gold appliances, no counter space whatsoever
Bot: Sounds like it's due for an upgrade. Do you have a sense of what you want?
User: we want to open it up. knock out the wall between the kitchen and living room, add an island, new cabinets, quartz counters. budget is 40-60k
Bot: That's a reasonable range for that scope of work. Have you started getting quotes?
User: not yet. we tried a handyman for some other stuff and it was fine but this feels like a bigger deal. we need someone who actually knows kitchen layout and cabinetry""",
                "expected_winner": "DreamKitchen Remodels",
            },
            "roof_leaking": {
                "conversation": """\
User: huge hailstorm last night and now there's water dripping from my living room ceiling. i can see shingles torn off from the yard
Bot: That needs immediate attention. Have you contacted your insurance company?
User: yeah they said get an estimate and file a claim. there's also a big dent in the ridge cap. i put a bucket under the drip but it's getting worse
Bot: You'll probably want someone who can get a tarp on it today before more rain comes.
User: that's what i'm thinking. i've never dealt with storm damage before, do roofers usually handle the insurance stuff too?""",
                "expected_winner": "StormShield Roofing",
            },
            "basement_flooding": {
                "conversation": """\
User: i came home and my basement has like two inches of water in it. there's water spraying from a pipe in the ceiling. i shut off the main valve but the damage is done
Bot: Good that you shut off the water. Do you know how old the plumbing is?
User: house is from 1955 so probably original. the pipe that burst looks pretty corroded
Bot: With a house that age there could be other pipes in similar condition.
User: yeah that's what i'm worried about. i need someone out here today if possible, this is a mess""",
                "expected_winner": "FlowFix Plumbing",
            },
        },
    },
}


# ── Seed advertisers ─────────────────────────────────────────────

@pytest.fixture(scope="module", autouse=True)
def register_all_advertisers(client):
    """Register all 16 advertisers (4 verticals × 4 each) at equal bids."""
    for vertical in VERTICALS.values():
        for name, intent, bid in vertical["advertisers"]:
            client.register(name=name, intent=intent, sigma=0.5, bid_price=bid, budget=500)


# ── Helpers ──────────────────────────────────────────────────────

def extract_intent(llm_client: anthropic.Anthropic, conversation: str) -> str:
    """Use Claude to distill a conversation into an 'X for Y' intent sentence."""
    response = llm_client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=100,
        messages=[
            {"role": "user", "content": f"{INTENT_PROMPT}\n\nConversation:\n{conversation}"},
        ],
    )
    return response.content[0].text.strip().strip("'\"")


# ── Build parametrize list ───────────────────────────────────────

def _build_scenarios():
    """Flatten verticals into (vertical, scenario_name, scenario_data) tuples."""
    scenarios = []
    for vertical_name, vertical in VERTICALS.items():
        for scenario_name, scenario_data in vertical["conversations"].items():
            test_id = f"{vertical_name}/{scenario_name}"
            scenarios.append((test_id, scenario_data))
    return scenarios


SCENARIOS = _build_scenarios()


# ── Tests ────────────────────────────────────────────────────────

@requires_api_key
@requires_server
@pytest.mark.parametrize("test_id,scenario", SCENARIOS, ids=[s[0] for s in SCENARIOS])
def test_specialist_beats_generalist(llm, client, test_id, scenario):
    """Conversation → LLM intent → ad server → specialist wins over generalist."""
    intent = extract_intent(llm, scenario["conversation"])

    ad = client.get_ad(intent)

    vertical = test_id.split("/")[0]
    generalist = VERTICALS[vertical]["advertisers"][0][0]

    winner_name = ad["winner"]["name"] if ad.get("winner") else "(no winner)"

    print(f"\n  [{test_id}]")
    print(f"  Intent:  {intent}")
    print(f"  Winner:  {winner_name} (${ad['payment']:.2f})")
    print(f"  Generalist in vertical: {generalist}")

    assert winner_name == scenario["expected_winner"], (
        f"Expected '{scenario['expected_winner']}' but got '{winner_name}'\n"
        f"  Intent: {intent}\n"
        f"  Generalist was: {generalist}"
    )


# ── Generalist-winning conversations ────────────────────────────
#
# These conversations are deliberately vague, broad, or straddle
# multiple specialist niches — so the generalist's wide framing
# ends up closer in embedding space than any single specialist.

GENERALIST_CONVERSATIONS = {
    "physical_therapy/general_stiffness": {
        "conversation": """\
User: my back and shoulders have been really stiff and achy lately. i sit at a desk like 10 hours a day
Bot: That's really common with desk jobs. Have you tried any stretching or exercise?
User: a little bit but it's not really helping. it's not like i injured myself or anything, i just feel tight and sore all the time
Bot: A physical therapist could help figure out what's going on and give you a plan.
User: yeah maybe. i just want to stop hurting honestly""",
        "expected_winner": "AllMotion PT",
    },
    "legal/small_business_question": {
        "conversation": """\
User: i'm opening a bakery with a friend and there's so much legal stuff i don't understand
Bot: What are you stuck on?
User: like everything? should we be an LLC or S-corp, we have a lease to sign, and we want to trademark the name. i don't even know if one lawyer does all that
Bot: A lot of business lawyers handle that kind of thing as a package.
User: ok good. we don't have any legal problems, we just need someone to help us set things up before we open""",
        "expected_winner": "CityWide Legal",
    },
    "financial/general_money_advice": {
        "conversation": """\
User: i just got a big raise and i have no idea what to do with the extra money. i'm 34
Bot: Congrats! What does your financial situation look like currently?
User: i have some savings, a 401k i set up years ago and never really looked at, no major debt. i just feel like i should be doing something smarter with my money
Bot: It might be worth talking to someone who can look at the whole picture.
User: yeah that's kind of what i want. i don't have a crisis or anything, i just want to get my act together""",
        "expected_winner": "PlanAhead Financial",
    },
    "home_services/general_maintenance": {
        "conversation": """\
User: we just bought our first house and there's a million little things that need fixing
Bot: That's pretty normal. What kind of stuff?
User: drywall patches, couple doors that stick, deck railing is wobbly, ceiling fan needs replacing. oh and the garage door opener is acting up. just a bunch of random stuff
Bot: Sounds like a handyman could knock most of that out in a day or two.
User: yeah that's what i'm thinking. nothing major, just need someone reliable who can work through a list""",
        "expected_winner": "HandyPro Services",
    },
}


def _build_generalist_scenarios():
    scenarios = []
    for test_id, data in GENERALIST_CONVERSATIONS.items():
        scenarios.append((test_id, data))
    return scenarios


GENERALIST_SCENARIOS = _build_generalist_scenarios()


@requires_api_key
@requires_server
@pytest.mark.parametrize("test_id,scenario", GENERALIST_SCENARIOS, ids=[s[0] for s in GENERALIST_SCENARIOS])
def test_generalist_wins_on_broad_query(llm, client, test_id, scenario):
    """Broad/vague conversation → generalist wins over specialists."""
    intent = extract_intent(llm, scenario["conversation"])

    ad = client.get_ad(intent)

    vertical = test_id.split("/")[0]
    generalist = VERTICALS[vertical]["advertisers"][0][0]
    specialists = [a[0] for a in VERTICALS[vertical]["advertisers"][1:]]

    winner_name = ad["winner"]["name"] if ad.get("winner") else "(no winner)"

    print(f"\n  [{test_id}]")
    print(f"  Intent:  {intent}")
    print(f"  Winner:  {winner_name} (${ad['payment']:.2f})")
    print(f"  Expected generalist: {generalist}")
    print(f"  Specialists: {', '.join(specialists)}")

    assert winner_name == scenario["expected_winner"], (
        f"Expected generalist '{scenario['expected_winner']}' but got '{winner_name}'\n"
        f"  Intent: {intent}\n"
        f"  Specialists were: {', '.join(specialists)}"
    )
