using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using TMPro;
using UnityEngine.SceneManagement;
using System.IO;
using UnityEngine.UI;

[System.Serializable]
public class Tim
{
    public List<float> t;
}

public class MenuManager : MonoBehaviour
{
    public GameObject mainPage, modePage, musicPage, settingsPage, tutorialPage;
    LevelInfo levelInfo = new LevelInfo();
    public TextMeshProUGUI[] percents;
    public Image[] clouds;
    public Image[] edges;
    public GameObject[] doneThings;
    public Color unlockedColor;
    public TextMeshProUGUI odblText, xpText, progres, rank, nextRank;
    bool unlockMode;
    public int unlockPrice = 300;
    public GameObject unlockImage;
    public Color[] rankColors;
    public string[] ranks;
    public int[] percentRanks;

    public void tutPageSwitch(bool x)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        mainPage.SetActive(!x);
        tutorialPage.SetActive(x);
    }

    public void modePageSwitch(bool x)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        mainPage.SetActive(!x);
        modePage.SetActive(x);
    }

    public void musicPageSwitch(bool x)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        modePage.SetActive(!x);
        musicPage.SetActive(x);
    }

    public void settingsPageSwitch(bool x)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        mainPage.SetActive(!x);
        settingsPage.SetActive(x);
    }

    private void Start()
    {
        SoundManager.Instance.PlayMenuMusic();
        odblText.text = SceneTransport.Instance.odb.ToString();
        int don = 0;
        foreach(var x in SceneTransport.Instance.done)
            if (x)
                don++;

        int prog = Mathf.RoundToInt((float)don / percents.Length * 100);
        progres.text = prog.ToString() + "%";
        int rankId = 0;
        while (rankId < percentRanks.Length && percentRanks[rankId] <= prog)
            rankId++;
        rankId--;
        rank.text = ranks[rankId];
        rank.color = rankColors[rankId];

        if (rankId == percentRanks.Length - 1) nextRank.gameObject.SetActive(false);
        else
        {
            nextRank.text = ranks[rankId + 1] + " (" + percentRanks[rankId + 1] + "%)";
            nextRank.color = rankColors[rankId + 1];
        }
        xpText.text = SceneTransport.Instance.xp.ToString();
    }

    private void Update()
    {
        if (modePage.activeInHierarchy && Input.GetKeyDown(KeyCode.Escape))
            modePageSwitch(false);
        if (musicPage.activeInHierarchy && Input.GetKeyDown(KeyCode.Escape))
            musicPageSwitch(false);
        if (settingsPage.activeInHierarchy && Input.GetKeyDown(KeyCode.Escape))
            settingsPageSwitch(false);
    }

    public void BuyUnlock()
    {
        if (SceneTransport.Instance.xp >= unlockPrice)
        {
            SoundManager.Instance.Sfx(SoundManager.Instance.buyUnlock);
            SceneTransport.Instance.xp -= unlockPrice;
            SceneTransport.Instance.odb++;

            odblText.text = SceneTransport.Instance.odb.ToString();
            xpText.text = SceneTransport.Instance.xp.ToString();

            Quests.Instance.GetInfo(-1, -1, false, false, true, -1, "", false, false, false);

            SceneTransport.Instance.Save();
        }
        else
        {
            xpText.GetComponent<Animator>().SetTrigger("error");
            SoundManager.Instance.Sfx(SoundManager.Instance.error);
        }
    }

    public void UnlockModeButton()
    {
        unlockMode = !unlockMode;
        if(unlockMode) SoundManager.Instance.Sfx(SoundManager.Instance.unlockModeOn);
        else SoundManager.Instance.Sfx(SoundManager.Instance.unlockModeOff);
        unlockImage.SetActive(unlockMode);
    }

    public void Normal()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        unlockMode = false;
        levelInfo.mode = 0;
        levelInfo.rival = false;
        musicPageSwitch(true);
        for (int i = 0; i < percents.Length; i++)
        {
            percents[i].gameObject.SetActive(true);
            percents[i].text = SceneTransport.Instance.percents[i].ToString() + "%";

            if (!SceneTransport.Instance.unlocked[i])
            {
                clouds[i].color = unlockedColor;
                if(i > 0 && i < 17)
                    edges[i].color = unlockedColor;
            }
            else
            {
                clouds[i].color = Color.white;
                if (i > 0 && i < 17)
                    edges[i].color = Color.white;
            }
            if (SceneTransport.Instance.done[i])
                doneThings[i].SetActive(true);
            else
                doneThings[i].SetActive(false);
                
        }
    }

    public void Hard()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        unlockMode = false;
        levelInfo.mode = 1;
        levelInfo.rival = false;
        musicPageSwitch(true);
        for (int i = 0; i < percents.Length; i++)
        {
            percents[i].gameObject.SetActive(true);
            percents[i].text = SceneTransport.Instance.percents1[i].ToString() + "%";

            if (!SceneTransport.Instance.unlocked[i])
            {
                clouds[i].color = unlockedColor;
                if (i > 0 && i < 17)
                    edges[i].color = unlockedColor;
            }
            else
            {
                clouds[i].color = Color.white;
                if (i > 0 && i < 17)
                    edges[i].color = Color.white;
            }

            if (SceneTransport.Instance.doneHard[i])
                doneThings[i].SetActive(true);
            else
                doneThings[i].SetActive(false);
        }
    }


    public void Mult()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        unlockMode = false;
        levelInfo.mode = 2;
        levelInfo.rival = true;
        musicPageSwitch(true);
        for (int i = 0; i < percents.Length; i++)
        {
            percents[i].gameObject.SetActive(false);

            if (!SceneTransport.Instance.unlocked[i])
            {
                clouds[i].color = unlockedColor;
                if (i > 0 && i < 17)
                    edges[i].color = unlockedColor;
            }
            else
            {
                clouds[i].color = Color.white;
                if (i > 0 && i < 17)
                    edges[i].color = Color.white;
            }

            if (SceneTransport.Instance.done[i] || SceneTransport.Instance.doneHard[i])
                doneThings[i].SetActive(true);
            else
                doneThings[i].SetActive(false);
        }
    }
    
    public void Multiplayer()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        unlockMode = false;
        levelInfo.mode = 3;
        levelInfo.rival = true;
        musicPageSwitch(true);
        for (int i = 0; i < percents.Length; i++)
        {
            percents[i].gameObject.SetActive(false);

            if (!SceneTransport.Instance.unlocked[i])
            {
                clouds[i].color = unlockedColor;
                if (i > 0 && i < 17)
                    edges[i].color = unlockedColor;
            }
            else
            {
                clouds[i].color = Color.white;
                if (i > 0 && i < 17)
                    edges[i].color = Color.white;
            }

            if (SceneTransport.Instance.done[i] || SceneTransport.Instance.doneHard[i])
                doneThings[i].SetActive(true);
            else
                doneThings[i].SetActive(false);
        }
    }

    public void MusicButton(int musicIndex)
    {
        if (unlockMode)
        {
            if (musicIndex == 0)
            {
                SoundManager.Instance.Sfx(SoundManager.Instance.error);
                return;
            }
            if (!SceneTransport.Instance.unlocked[musicIndex] && SceneTransport.Instance.odb > 0 && (musicIndex == 1 || musicIndex == 5 || musicIndex == 9 || musicIndex == 13 || SceneTransport.Instance.unlocked[musicIndex - 1]) || (SceneTransport.Instance.unlocked[musicIndex + 1] && musicIndex != 16 && musicIndex != 17 && musicIndex != 12 && musicIndex != 8 && musicIndex != 4) || ((musicIndex == 16 || musicIndex == 12 || musicIndex == 8 || musicIndex == 4) && SceneTransport.Instance.unlocked[17]) || musicIndex == 17)
            {
                if (musicIndex == 17 && (!SceneTransport.Instance.unlocked[16] || !SceneTransport.Instance.unlocked[12] || !SceneTransport.Instance.unlocked[8] || !SceneTransport.Instance.unlocked[4]))
                    return;

                Quests.Instance.GetInfo(-1, -1, false, false, false, -1, "", true, false, false);
                clouds[musicIndex].color = Color.white;
                SceneTransport.Instance.odb--;
                SceneTransport.Instance.unlocked[musicIndex] = true;
                odblText.text = SceneTransport.Instance.odb.ToString();
                SceneTransport.Instance.Save();
                SoundManager.Instance.Sfx(SoundManager.Instance.unlock);
            }
            else
            {
                SoundManager.Instance.Sfx(SoundManager.Instance.error);
                if (SceneTransport.Instance.odb == 0)
                    odblText.GetComponent<Animator>().SetTrigger("error");
            }
            return;
        }
        if (!SceneTransport.Instance.unlocked[musicIndex])
        {
            SoundManager.Instance.Sfx(SoundManager.Instance.error);
            return;
        }
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        levelInfo.musicIndex = musicIndex;
        List<float> times = new List<float>();
        List<float> times1 = new List<float>();

        string s = File.ReadAllText(Application.persistentDataPath + "/" + musicIndex.ToString() + ".json");
        Tim lista = JsonUtility.FromJson<Tim>(s);
        times = lista.t;

        if (levelInfo.mode == 1)
        {
            s = File.ReadAllText(Application.persistentDataPath + "/" + musicIndex.ToString() + "B.json");
            lista = JsonUtility.FromJson<Tim>(s);
            times1 = lista.t;
        }
        else
            times1 = times;

        levelInfo.times = times;
        levelInfo.times1 = times1;

        File.WriteAllText(Application.persistentDataPath + "/levelinfo.json", JsonUtility.ToJson(levelInfo));
        SoundManager.Instance.StopMusic();
        SceneManager.LoadScene("SampleScene");
    }
}
