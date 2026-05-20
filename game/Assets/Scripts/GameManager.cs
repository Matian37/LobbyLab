using System;
using System.Collections;
using System.Collections.Generic;
using TMPro;
using UnityEngine;
using UnityEngine.SceneManagement;
using UnityEngine.UIElements;

public class GameManager : MonoBehaviour
{
    bool didMusicStart = false;
    int mode;
    bool win = false;
    string startHour = "";
    int indx;
    float songDuration;

    bool rival;
    public float timeBeforeMusic = 5;
    float time = 0;
    public TextMeshProUGUI text, loseText, multiWinText;
    public GameObject winInfo, loseInfo, multiWinInfo;
    public string leftWinText, rightWinText;
    public bool multiplayer; //67

    public void GameStart(bool _rival, int _mode, int _indx, float _songDur)
    {
        startHour = DateTime.Now.Hour + ":" + DateTime.Now.Minute;
        rival = _rival;
        mode = _mode;
        indx = _indx;
        songDuration = _songDur;
    }

    public void Lose(bool lewyWin)
    {
        Save();
        SoundManager.Instance.StopMusic();
        if (rival)
        {
            loseInfo.SetActive(true);
            if (lewyWin) loseText.text = leftWinText;
            else loseText.text = rightWinText;
            Time.timeScale = 0;
        }
        else
            SceneManager.LoadScene("SampleScene");
    }

    private void Awake()
    {
        Time.timeScale = 1;
    }

    void ShowWin()
    {
        win = true;
        if (mode == 0 && !SceneTransport.Instance.done[indx])
        {
            SceneTransport.Instance.odb++;
            SceneTransport.Instance.xp += 100;
            SceneTransport.Instance.done[indx] = true;
        }
        else if (mode == 1 && !rival && !SceneTransport.Instance.doneHard[indx])
        {
            SceneTransport.Instance.odb++;
            SceneTransport.Instance.xp += 100;
            SceneTransport.Instance.doneHard[indx] = true;
        }
        Save();
        Time.timeScale = 0;
        winInfo.SetActive(true);
    }

    public void MultEndGame(string message)
    {
        multiWinInfo.SetActive(true);
        multiWinText.text = message;
        SoundManager.Instance.StopMusic();
        Time.timeScale = 0;
    }

    public bool didGameStart = false;
    private void Update()
    {
        if (multiplayer && !didGameStart) return;
        time += Time.deltaTime;

        if (time >= timeBeforeMusic && !didMusicStart)
            startMusic();
        if (time >= timeBeforeMusic && time - timeBeforeMusic >= songDuration + 2 && !multiplayer)
            ShowWin();
        if (time >= timeBeforeMusic)
            text.text = Mathf.RoundToInt((time - timeBeforeMusic) / (songDuration + 2) * 100).ToString() + "%";
    }

    void QuestInfo()
    {
        int perc = Mathf.RoundToInt((time - timeBeforeMusic) / (songDuration + 2) * 100);
        bool rekord = false;

        if (mode == 0 && SceneTransport.Instance.percents[indx] < perc)
            rekord = true;
        else if (mode == 1 && SceneTransport.Instance.percents1[indx] < perc)
            rekord = true;

        bool muted = SceneTransport.Instance.volume == 0 && win;
        bool bindy = SceneTransport.Instance.binds[0] == KeyCode.D && SceneTransport.Instance.binds[1] == KeyCode.A && mode == 0 && win;
        Quests.Instance.GetInfo(perc, time, rekord, rival && win, false, indx, startHour, false, muted, bindy);
    }

    public void Save()
    {
        QuestInfo();
        if (rival) return;

        int perc = Mathf.RoundToInt((time - timeBeforeMusic) / (songDuration + 2) * 100);

        if (mode == 0 && SceneTransport.Instance.percents[indx] < perc)
            SceneTransport.Instance.percents[indx] = perc;
        else if (mode == 1 && SceneTransport.Instance.percents1[indx] < perc)
            SceneTransport.Instance.percents1[indx] = perc;

        SceneTransport.Instance.Save();
    }

    private void OnApplicationQuit()
    {
        Save();
    }
    void startMusic()
    {
        didMusicStart = true;
        SoundManager.Instance.PlaySong(indx);
    }
}
