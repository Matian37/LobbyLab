using System.Collections;
using System.Collections.Generic;
using System;
using UnityEngine;

public class Quests : MonoBehaviour
{
    public static Quests Instance { get; private set; }
    public string[] quests;
    public int[] rewards;
    public int[] ind;
    public bool[] collected, collect;

    private void Awake()
    {
        if (Instance is null)
        {
            Instance = this;
        }
        else
        {
            Destroy(gameObject);
        }

        DontDestroyOnLoad(gameObject);

        DateTime dt = DateTime.Now;

        int seed = dt.Year + dt.Month * 7 + dt.Day * 49;
        UnityEngine.Random.InitState(seed);

        ind = new int[2];
        ind[0] = UnityEngine.Random.Range(0, quests.Length);
        ind[1] = ind[0];
        while (ind[1] == ind[0])
            ind[1] = UnityEngine.Random.Range(0, quests.Length);
    }

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }

    public void GetInfo(int percent, float deathTime, bool rekord, bool multWin, bool piorko, int musicIndex, string startHour, bool unlock, bool muted, bool bindy)
    {
        for (int i = 0; i < ind.Length; i++)
        {
            if (ind[i] == 0) //67%
            {
                if (percent == 67)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 1) //zgin w 1 sek
            {
                if (deathTime <= 1 && deathTime != -1)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 2) //new record
            {
                if(rekord)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 3)
            {
                if(deathTime >= 60)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 4)
            {
                if (multWin)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 5)
            {
                if (piorko) //nowy skin
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 6)
            {
                if (piorko)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 7)
            {
                if (musicIndex == 7 && startHour == "21:37")
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 8)
            {
                if (unlock)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 9)
            {
                if (muted)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 10)
            {
                if (bindy)
                {
                    CompleteQuest(i);
                }
            }
            else if (ind[i] == 11)
            {
                if (percent == 99)
                {
                    CompleteQuest(i);
                }
            }
        }
    }

    void CompleteQuest(int i)
    {
        collect[i] = true;
        QuestDisplay qd = FindObjectOfType<QuestDisplay>();
        if(qd != null)
            qd.Refresh();
    }
}
