using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class LivesManager : MonoBehaviour
{
    public int lives = 3;
    public Server ser;
    public bool isLeft;
    public float timeToRegen = 20;
    float time = 0;
    public void EraseLife()
    {
        time = 0;
        lives--;
        if (lives == 0)
            ser.Lose(isLeft);
    }

    public void Update()
    {
        time += Time.deltaTime;
        if (time >= timeToRegen)
        {
            lives = Mathf.Min(lives + 1, 3);
            time = 0;
        }
    }
}
