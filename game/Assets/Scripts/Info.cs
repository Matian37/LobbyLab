using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using System.IO;

[System.Serializable]
public class LevelInfo
{
    public List<float> times, times1;
    public int musicIndex;
    public bool rival;
    public int mode;
}

public class Info : MonoBehaviour
{
    public GameObject[] modes;
    public GameObject[] turnOnRivalry;

    public PlayerController altLeft, altRight;

    private void Awake()
    {
        string s = File.ReadAllText(Application.persistentDataPath + "/levelinfo.json");
        LevelInfo info = JsonUtility.FromJson<LevelInfo>(s);

        if (info.mode == 0) modes[0].SetActive(true);
        else modes[1].SetActive(true);

        if(info.rival || info.mode == 3)
            foreach (GameObject g in turnOnRivalry)
                g.SetActive(true);
        if(info.mode == 1)
        {
            altRight.livesManager = altLeft.livesManager;
        }

        foreach (var x in FindObjectsOfType<ObstaclesGen>())
        {
            if(x.gameObject.activeInHierarchy)
            {
                if (info.mode == 1 && x.altLeft)
                    x.GetTimes(new List<float>(info.times1), info.musicIndex, info.rival, info.mode);
                else
                    x.GetTimes(new List<float>(info.times), info.musicIndex, info.rival, info.mode);
            }
        }
    }
}
