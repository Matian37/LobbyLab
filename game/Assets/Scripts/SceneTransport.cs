using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using System.IO;
using System;

public class Save
{
    public int xp, odbl;
    public List<int> percents, percents1;
    public List<bool> unl;
    public List<bool> done, collectedQuests, collectQuests, doneHard;
    public float volume;
    public List<KeyCode> binds = new List<KeyCode>();
    public int dateHash;
    public Save()
    {
        dateHash = 0;
        volume = 1;
        percents = new List<int>();
        percents1 = new List<int>();
        doneHard = new List<bool>();
        collectedQuests = new List<bool>();
        done = new List<bool>();
        unl = new List<bool>();
        collectQuests = new List<bool>();
        xp = odbl = 0;
        binds.Add(KeyCode.A);
        binds.Add(KeyCode.D);
        binds.Add(KeyCode.LeftArrow);
        binds.Add(KeyCode.RightArrow);
        for (int i = 0; i < 100; i++)
        {
            percents.Add(0);
            percents1.Add(0);
            unl.Add(false);
            done.Add(false);
            doneHard.Add(false);
        }
        unl[0] = true;

        for(int i = 0; i < 2; i++)
        {
            collectedQuests.Add(false);
            collectQuests.Add(false);
        }
    }
}

public class SceneTransport : MonoBehaviour
{
    public static SceneTransport Instance { get; set; }

    public List<bool> unlocked = new List<bool>();
    public List<bool> done = new List<bool>();
    public List<bool> doneHard = new List<bool>();
    public List<int> percents = new List<int>();
    public List<int> percents1 = new List<int>();
    public List<KeyCode> binds = new List<KeyCode>();
    public int xp = 0, odb = 0;
    public float volume = 1;
    private void Awake()
    {
        if (Instance is null)
            Instance = this;
        else
            Destroy(gameObject);

        DontDestroyOnLoad(this);
        Save save;
        if (!File.Exists(Application.persistentDataPath + "/save.json"))
        {
            save = new Save();
            File.WriteAllText(Application.persistentDataPath + "/save.json", JsonUtility.ToJson(save));
        }
        else
        {
            string s = File.ReadAllText(Application.persistentDataPath + "/save.json");
            save = JsonUtility.FromJson<Save>(s);
        }

        volume = save.volume;
        binds = save.binds;
        done = save.done;
        doneHard = save.doneHard;
        unlocked = save.unl;
        percents = save.percents;
        percents1 = save.percents1;
        xp = save.xp;
        odb = save.odbl;

        DateTime dt = DateTime.Now;
        int hasz = dt.Year + dt.Month * 7 + dt.Day * 49;
        if(hasz == save.dateHash)
        {
            Quests.Instance.collect = save.collectQuests.ToArray();
            Quests.Instance.collected = save.collectedQuests.ToArray();
        }

        Save();
    }

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }

    public void Save()
    {
        string s = File.ReadAllText(Application.persistentDataPath + "/save.json");
        Save save = JsonUtility.FromJson<Save>(s);

        save.volume = volume;
        save.binds = binds;
        save.done = done;
        save.doneHard = doneHard;
        save.percents = percents;
        save.percents1 = percents1;
        save.xp = xp;
        save.odbl = odb;
        save.unl = unlocked;
        save.dateHash = DateTime.Now.Year + DateTime.Now.Month * 7 + DateTime.Now.Day * 49;
        save.collectedQuests = new List<bool>(Quests.Instance.collected);
        save.collectQuests = new List<bool>(Quests.Instance.collect);

        File.WriteAllText(Application.persistentDataPath + "/save.json", JsonUtility.ToJson(save));
    }
}
