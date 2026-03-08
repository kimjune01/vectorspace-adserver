import type { ChatMessage } from "@vectorspace/sdk";

export interface Conversation {
  label: string;
  offTopic?: boolean;
  messages: ChatMessage[];
}

export const conversations: Conversation[] = [
  {
    label: "Anxiety & Sleep",
    messages: [
      { role: "user", content: "I've been feeling really anxious lately and I can't sleep at night." },
      { role: "assistant", content: "I'm sorry to hear that. Anxiety and sleep problems often go hand in hand. How long has this been going on?" },
      { role: "user", content: "A few weeks now. I lie awake worrying about work and everything just spirals." },
      { role: "assistant", content: "That sounds really tough. Have you tried any relaxation techniques before bed, like deep breathing or meditation?" },
      { role: "user", content: "Not really. I just end up scrolling my phone which probably makes it worse." },
      { role: "assistant", content: "Screen time before bed can definitely make things harder. Would you be open to exploring some structured approaches to manage the anxiety?" },
    ],
  },
  {
    label: "High Cholesterol",
    messages: [
      { role: "user", content: "My doctor just told me my cholesterol is too high." },
      { role: "assistant", content: "That's a common concern. Did your doctor mention whether it was your LDL, HDL, or total cholesterol that was high?" },
      { role: "user", content: "LDL mostly. They said I should change my diet before they consider medication." },
      { role: "assistant", content: "Diet changes can make a real difference. Things like reducing saturated fats, eating more fiber, and adding omega-3s are commonly recommended." },
      { role: "user", content: "I honestly don't know where to start with meal planning. I eat out a lot." },
      { role: "assistant", content: "Starting from scratch can feel overwhelming. A structured nutrition plan could help you transition gradually without feeling deprived." },
    ],
  },
  {
    label: "Back Pain",
    messages: [
      { role: "user", content: "My back pain is making it hard to work. I sit at a desk all day." },
      { role: "assistant", content: "Chronic back pain from desk work is very common. How long have you been dealing with this?" },
      { role: "user", content: "Months now. It started as occasional tightness but now it's constant." },
      { role: "assistant", content: "It sounds like it's been getting progressively worse. Have you seen a doctor or physical therapist about it?" },
      { role: "user", content: "I went to my doctor and they said physical therapy would help, but I can never make it to appointments." },
      { role: "assistant", content: "That's a common barrier. There are actually some great virtual physical therapy options now that let you do guided exercises from home." },
    ],
  },
  {
    label: "Loneliness After Moving",
    messages: [
      { role: "user", content: "I just moved to a new city and I feel really lonely." },
      { role: "assistant", content: "Moving to a new place can be really isolating. It takes time to build a social network from scratch." },
      { role: "user", content: "Yeah, I left all my friends behind. I work remotely so I don't even have coworkers to hang out with." },
      { role: "assistant", content: "Remote work can make it even harder to meet people organically. Have you tried any local meetups or activities?" },
      { role: "user", content: "I've thought about it but I'm kind of introverted. Walking into a room of strangers feels impossible." },
      { role: "assistant", content: "That's completely understandable. Sometimes one-on-one connections feel more manageable than group settings." },
    ],
  },
  {
    label: "Relationship Conflict",
    messages: [
      { role: "user", content: "My partner and I keep fighting about everything. It feels like we can't communicate." },
      { role: "assistant", content: "Communication breakdowns are one of the most common relationship challenges. What kinds of things trigger the arguments?" },
      { role: "user", content: "Money, chores, how we spend our time. It's like we're on completely different pages." },
      { role: "assistant", content: "It sounds like there might be some underlying issues around expectations and priorities. Have you two ever tried talking things through with a neutral third party?" },
      { role: "user", content: "We talked about couples therapy but we can't agree on that either. Plus scheduling is hard." },
      { role: "assistant", content: "There are actually some flexible options now — online platforms that let you work with a therapist on your own schedule." },
    ],
  },
  {
    label: "Quitting Smoking",
    messages: [
      { role: "user", content: "I'm trying to quit smoking but I keep relapsing." },
      { role: "assistant", content: "Quitting smoking is really hard — most people need multiple attempts. How long have you been trying?" },
      { role: "user", content: "On and off for a year. I'll go a week or two and then stress hits and I'm back at it." },
      { role: "assistant", content: "Stress is one of the biggest triggers for relapse. Have you tried any structured quit programs or nicotine replacement?" },
      { role: "user", content: "I tried patches but they didn't really help with the behavioral part. Like the habit of stepping outside." },
      { role: "assistant", content: "You're right that there's a strong behavioral component. Programs that combine cognitive behavioral techniques with tracking can address both the physical and psychological aspects." },
    ],
  },
  {
    label: "Just Chatting: Recipes",
    offTopic: true,
    messages: [
      { role: "user", content: "What's a good recipe for banana bread?" },
      { role: "assistant", content: "Classic banana bread is great! You'll need 3 ripe bananas, 1/3 cup melted butter, 3/4 cup sugar, 1 egg, 1 teaspoon vanilla, 1 teaspoon baking soda, and 1.5 cups flour." },
      { role: "user", content: "Can I add chocolate chips?" },
      { role: "assistant", content: "Absolutely! Fold in about 1 cup of chocolate chips into the batter. Dark chocolate works especially well with the banana flavor." },
      { role: "user", content: "How long do I bake it?" },
      { role: "assistant", content: "Bake at 350°F (175°C) for about 60-65 minutes. It's done when a toothpick inserted in the center comes out clean." },
    ],
  },
  {
    label: "Just Chatting: Movies",
    offTopic: true,
    messages: [
      { role: "user", content: "Can you recommend a good movie to watch tonight?" },
      { role: "assistant", content: "What genre are you in the mood for? Comedy, thriller, drama, sci-fi?" },
      { role: "user", content: "Something lighthearted. Maybe a comedy." },
      { role: "assistant", content: "How about 'The Grand Budapest Hotel'? It's quirky, funny, and visually stunning. Wes Anderson at his best." },
      { role: "user", content: "Oh I love Wes Anderson! Any other suggestions?" },
      { role: "assistant", content: "If you like Anderson, try 'Moonrise Kingdom' or 'The French Dispatch'. They have that same whimsical style and dry humor." },
    ],
  },
];
